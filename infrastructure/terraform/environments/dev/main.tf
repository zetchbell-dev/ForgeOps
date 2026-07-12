# This file only wires modules together with environment-specific variables
# (M1 §2: "environments/<env>/main.tf only wires modules together... it
# contains no resource logic itself"). Anything that looks like a decision
# beyond "which value goes into which module input" belongs in a module, not
# here.

module "vpc" {
  source = "../../modules/vpc"

  environment        = var.environment
  project_name       = var.project_name
  cidr_block         = var.vpc_cidr_block
  az_count           = var.az_count
  single_nat_gateway = var.single_nat_gateway
  tags               = var.tags
}

module "eks" {
  source = "../../modules/eks"

  environment  = var.environment
  project_name = var.project_name

  cluster_name    = var.cluster_name
  cluster_version = var.cluster_version

  vpc_id             = module.vpc.vpc_id
  private_subnet_ids = module.vpc.private_subnet_ids

  endpoint_public_access       = true
  endpoint_public_access_cidrs = var.endpoint_public_access_cidrs

  node_instance_types = var.node_instance_types
  node_desired_size   = var.node_desired_size
  node_min_size       = var.node_min_size
  node_max_size       = var.node_max_size

  tags = var.tags
}

module "iam" {
  source = "../../modules/iam"

  environment  = var.environment
  project_name = var.project_name

  cluster_name      = module.eks.cluster_name
  oidc_provider_arn = module.eks.oidc_provider_arn
  oidc_provider_url = module.eks.oidc_provider_url

  tags = var.tags
}

# ---------------------------------------------------------------------------
# ECR is instantiated in environments/dev ONLY, not repeated in
# environments/staging or environments/prod.
#
# Why: an ECR repository is an account/region-level registry resource, not a
# per-environment one — M4 §4's promotion flow ("bump overlays/staging tag to
# the same SHA already proven in dev") only makes sense if dev, staging, and
# prod are all pulling from the *same* repository. If each environment's root
# module called module.ecr independently, staging's and prod's applies would
# either collide on the repository name (ECR names are unique per
# account+region, not per Terraform state) or silently create the same
# resource in two states, which is exactly the kind of split-brain state
# ownership M1 §3's per-environment state design is meant to prevent, not
# cause.
#
# staging/prod instead read repository info via a `terraform_remote_state`
# data source pointed at this state file (see environments/staging/data.tf
# once that root module exists) — read-only, never re-declaring the resource.
# ---------------------------------------------------------------------------

module "ecr" {
  source = "../../modules/ecr"

  environment      = var.environment
  project_name     = var.project_name
  repository_names = var.repository_names

  tags = var.tags
}
