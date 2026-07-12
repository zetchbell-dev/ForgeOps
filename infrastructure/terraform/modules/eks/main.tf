locals {
  name_prefix = "${var.project_name}-${var.environment}"

  mandatory_tags = {
    Environment = var.environment
    ManagedBy   = "terraform"
    Project     = var.project_name
  }
  tags = merge(local.mandatory_tags, var.tags)

  # Node group's underlying ASG must carry these two tags for module.iam's
  # cluster-autoscaler permission policy to apply to it (condition key
  # autoscaling:ResourceTag/k8s.io/cluster-autoscaler/<cluster_name> = "owned",
  # see modules/iam/main.tf). EKS-managed node groups propagate the `tags`
  # argument through to the backing ASG, so setting them here is sufficient —
  # no separate ASG resource to tag directly, since this module intentionally
  # uses a managed node group rather than hand-rolled Auto Scaling Group + launch
  # template (less to maintain, AWS handles AMI/patch rollout mechanics).
  node_group_tags = {
    "k8s.io/cluster-autoscaler/${var.cluster_name}" = "owned"
    "k8s.io/cluster-autoscaler/enabled"              = "true"
  }
}

# ---------------------------------------------------------------------------
# KMS key for Kubernetes Secrets envelope encryption (optional, default on).
# Encrypts Secret objects in etcd with a customer-managed key on top of EKS's
# default at-rest encryption — see variable description for rationale.
# ---------------------------------------------------------------------------

resource "aws_kms_key" "eks_secrets" {
  count = var.enable_secrets_encryption ? 1 : 0

  description             = "Envelope encryption key for ${local.name_prefix} EKS cluster Kubernetes Secrets."
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = merge(local.tags, {
    Name = "${local.name_prefix}-eks-secrets"
  })
}

resource "aws_kms_alias" "eks_secrets" {
  count = var.enable_secrets_encryption ? 1 : 0

  name          = "alias/${local.name_prefix}-eks-secrets"
  target_key_id = aws_kms_key.eks_secrets[0].key_id
}

# ---------------------------------------------------------------------------
# EKS cluster IAM role (control plane service role)
# ---------------------------------------------------------------------------

data "aws_iam_policy_document" "cluster_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "cluster" {
  name               = "${local.name_prefix}-eks-cluster"
  assume_role_policy = data.aws_iam_policy_document.cluster_assume_role.json

  tags = merge(local.tags, {
    Name = "${local.name_prefix}-eks-cluster"
  })
}

# AWS-managed policy — this is the control plane's own permission to manage
# ENIs/ELBs/etc. on our behalf, not a workload-facing grant, so the AWS-managed
# policy (rather than a hand-rolled least-privilege document) is the correct
# and standard choice here.
resource "aws_iam_role_policy_attachment" "cluster_eks_cluster_policy" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

# ---------------------------------------------------------------------------
# CloudWatch log group for control plane logs (Section: enabled_cluster_log_types)
# Must exist before the cluster ships logs to it, and named to match EKS's
# fixed /aws/eks/<cluster-name>/cluster convention.
# ---------------------------------------------------------------------------

resource "aws_cloudwatch_log_group" "cluster" {
  name              = "/aws/eks/${var.cluster_name}/cluster"
  retention_in_days = var.log_retention_days

  tags = merge(local.tags, {
    Name = "${local.name_prefix}-eks-cluster-logs"
  })
}

# ---------------------------------------------------------------------------
# EKS cluster (control plane)
#
# subnet_ids uses private subnets only, for both control-plane cross-account
# ENIs and (separately, below) the node group — per M1 §6, nodes never sit in
# a public subnet. This is orthogonal to endpoint_public_access, which governs
# whether the API server's public endpoint is reachable from outside the VPC,
# not where the control plane's own ENIs live.
# ---------------------------------------------------------------------------

resource "aws_eks_cluster" "this" {
  name     = var.cluster_name
  role_arn = aws_iam_role.cluster.arn
  version  = var.cluster_version

  vpc_config {
    subnet_ids              = var.private_subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = var.endpoint_public_access
    public_access_cidrs     = var.endpoint_public_access ? var.endpoint_public_access_cidrs : ["0.0.0.0/32"] # 0.0.0.0/32 is unreachable — effectively "no public CIDRs" when public access is off, since the API still requires a non-empty list
  }

  enabled_cluster_log_types = var.enabled_cluster_log_types

  dynamic "encryption_config" {
    for_each = var.enable_secrets_encryption ? [1] : []
    content {
      provider {
        key_arn = aws_kms_key.eks_secrets[0].arn
      }
      resources = ["secrets"]
    }
  }

  tags = merge(local.tags, {
    Name = local.name_prefix
  })

  depends_on = [
    aws_iam_role_policy_attachment.cluster_eks_cluster_policy,
    aws_cloudwatch_log_group.cluster,
  ]
}

# ---------------------------------------------------------------------------
# OIDC provider — what makes IRSA (module.iam) possible. Created here because
# the OIDC issuer URL and its TLS certificate thumbprint are properties of
# this specific cluster; module.iam takes the resulting ARN/URL as inputs
# rather than creating this itself, keeping the two modules decoupled at
# authoring time (see modules/iam/README.md).
# ---------------------------------------------------------------------------

data "tls_certificate" "cluster_oidc" {
  url = aws_eks_cluster.this.identity[0].oidc[0].issuer
}

resource "aws_iam_openid_connect_provider" "cluster" {
  url             = aws_eks_cluster.this.identity[0].oidc[0].issuer
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.cluster_oidc.certificates[0].sha1_fingerprint]

  tags = merge(local.tags, {
    Name = "${local.name_prefix}-eks-oidc"
  })
}

# ---------------------------------------------------------------------------
# Managed node group — worker nodes, private subnets only.
# ---------------------------------------------------------------------------

data "aws_iam_policy_document" "node_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "node" {
  name               = "${local.name_prefix}-eks-node"
  assume_role_policy = data.aws_iam_policy_document.node_assume_role.json

  tags = merge(local.tags, {
    Name = "${local.name_prefix}-eks-node"
  })
}

# Three AWS-managed policies form the standard, minimum EKS worker node
# permission set: talk to the control plane as a kubelet, run the VPC CNI
# (attach/detach ENIs and IPs for pod networking), and pull images from ECR.
# Least-privilege here means using exactly these three, not layering on
# broader EC2/S3 permissions nodes don't need.
resource "aws_iam_role_policy_attachment" "node_worker_policy" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
}

resource "aws_iam_role_policy_attachment" "node_cni_policy" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
}

resource "aws_iam_role_policy_attachment" "node_ecr_read_only" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_eks_node_group" "this" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "${local.name_prefix}-default"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = var.private_subnet_ids # never public — M1 §6

  instance_types = var.node_instance_types
  capacity_type  = var.node_capacity_type
  disk_size      = var.node_disk_size

  scaling_config {
    desired_size = var.node_desired_size
    min_size     = var.node_min_size
    max_size     = var.node_max_size
  }

  update_config {
    max_unavailable = var.node_max_unavailable
  }

  labels = {
    "node-pool" = "default"
  }

  tags = merge(local.tags, local.node_group_tags, {
    Name = "${local.name_prefix}-eks-node-default"
  })

  # Terraform can't safely resolve a mid-rollout size drift caused by the
  # cluster-autoscaler adjusting desired_size at runtime, so scaling_config's
  # desired_size is intentionally excluded from plan comparisons — Terraform
  # still manages min/max, autoscaler owns desired within that band.
  lifecycle {
    ignore_changes = [scaling_config[0].desired_size]
  }

  depends_on = [
    aws_iam_role_policy_attachment.node_worker_policy,
    aws_iam_role_policy_attachment.node_cni_policy,
    aws_iam_role_policy_attachment.node_ecr_read_only,
  ]
}

# ---------------------------------------------------------------------------
# Core EKS add-ons — vpc-cni, coredns, kube-proxy. These are the three
# baseline add-ons every cluster needs regardless of workload; anything
# workload-specific (e.g. an EBS CSI driver for persistent volumes) is
# deferred to the milestone that first needs persistent storage, per the
# same "don't provision what nothing needs yet" principle M1 §1 applies to
# RDS/Vault — avoids unused surface area.
# ---------------------------------------------------------------------------

resource "aws_eks_addon" "vpc_cni" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "vpc-cni"
  resolve_conflicts_on_update = "OVERWRITE"

  tags = local.tags

  depends_on = [aws_eks_node_group.this]
}

resource "aws_eks_addon" "coredns" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "coredns"
  resolve_conflicts_on_update = "OVERWRITE"

  tags = local.tags

  # CoreDNS pods schedule onto worker nodes, so the node group must exist first.
  depends_on = [aws_eks_node_group.this]
}

resource "aws_eks_addon" "kube_proxy" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "kube-proxy"
  resolve_conflicts_on_update = "OVERWRITE"

  tags = local.tags

  depends_on = [aws_eks_node_group.this]
}
