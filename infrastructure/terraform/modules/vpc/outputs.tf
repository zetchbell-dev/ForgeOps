output "vpc_id" {
  description = "ID of the created VPC."
  value       = aws_vpc.this.id
}

output "vpc_cidr_block" {
  description = "CIDR block of the VPC."
  value       = aws_vpc.this.cidr_block
}

output "public_subnet_ids" {
  description = "IDs of the public subnets (one per AZ). Used for ALB placement."
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "IDs of the private subnets (one per AZ). Used for EKS node groups per M1 §6 (nodes never in public subnets)."
  value       = aws_subnet.private[*].id
}

output "availability_zones" {
  description = "AZs the VPC's subnets are spread across."
  value       = local.azs
}

output "nat_gateway_ids" {
  description = "IDs of the provisioned NAT Gateway(s)."
  value       = aws_nat_gateway.this[*].id
}

output "internet_gateway_id" {
  description = "ID of the Internet Gateway."
  value       = aws_internet_gateway.this.id
}
