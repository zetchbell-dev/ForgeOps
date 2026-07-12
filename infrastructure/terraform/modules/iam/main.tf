locals {
  name_prefix = "${var.project_name}-${var.environment}"

  mandatory_tags = {
    Environment = var.environment
    ManagedBy   = "terraform"
    Project     = var.project_name
  }
  tags = merge(local.mandatory_tags, var.tags)

  cluster_autoscaler_sa_subject = "system:serviceaccount:${var.cluster_autoscaler_namespace}:${var.cluster_autoscaler_service_account_name}"
}

# ---------------------------------------------------------------------------
# Cluster Autoscaler IRSA role
#
# Trust policy is scoped to the exact namespace:serviceaccount subject via the
# OIDC provider's `sub` condition key — this is what makes IRSA meaningfully
# different from a node-level IAM role (M1 §6): only pods running as this
# specific ServiceAccount can assume this role, not every pod on the node.
# ---------------------------------------------------------------------------

data "aws_iam_policy_document" "cluster_autoscaler_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [var.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${var.oidc_provider_url}:sub"
      values   = [local.cluster_autoscaler_sa_subject]
    }

    condition {
      test     = "StringEquals"
      variable = "${var.oidc_provider_url}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "cluster_autoscaler" {
  name               = "${local.name_prefix}-cluster-autoscaler"
  assume_role_policy = data.aws_iam_policy_document.cluster_autoscaler_assume_role.json

  tags = merge(local.tags, {
    Name = "${local.name_prefix}-cluster-autoscaler"
  })
}

# Permission policy scoped to the actions the AWS-documented cluster-autoscaler
# IAM policy requires, restricted to this cluster's autoscaling groups via the
# k8s.io/cluster-autoscaler/<cluster-name> tag condition — not a blanket
# autoscaling:* grant across every ASG in the account.
data "aws_iam_policy_document" "cluster_autoscaler_permissions" {
  statement {
    sid    = "ClusterAutoscalerDescribe"
    effect = "Allow"
    actions = [
      "autoscaling:DescribeAutoScalingGroups",
      "autoscaling:DescribeAutoScalingInstances",
      "autoscaling:DescribeLaunchConfigurations",
      "autoscaling:DescribeScalingActivities",
      "autoscaling:DescribeTags",
      "ec2:DescribeInstanceTypes",
      "ec2:DescribeLaunchTemplateVersions",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "ClusterAutoscalerMutate"
    effect = "Allow"
    actions = [
      "autoscaling:SetDesiredCapacity",
      "autoscaling:TerminateInstanceInAutoScalingGroup",
      "autoscaling:UpdateAutoScalingGroup",
    ]
    resources = ["*"]

    condition {
      test     = "StringEquals"
      variable = "autoscaling:ResourceTag/k8s.io/cluster-autoscaler/${var.cluster_name}"
      values   = ["owned"]
    }
  }
}

resource "aws_iam_policy" "cluster_autoscaler" {
  name        = "${local.name_prefix}-cluster-autoscaler"
  description = "Permissions for the Kubernetes cluster-autoscaler to manage node group scaling for ${var.cluster_name}, scoped to ASGs tagged for this cluster."
  policy      = data.aws_iam_policy_document.cluster_autoscaler_permissions.json

  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "cluster_autoscaler" {
  role       = aws_iam_role.cluster_autoscaler.name
  policy_arn = aws_iam_policy.cluster_autoscaler.arn
}
