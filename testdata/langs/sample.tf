terraform {
  required_version = ">= 1.0"
}

provider "aws" {
  region = "us-east-1"
}

variable "region" {
  type    = string
  default = "us-east-1"
}

resource "aws_s3_bucket" "logs" {
  bucket = "logs"
}

data "aws_ami" "ubuntu" {
  most_recent = true
}

module "vpc" {
  source = "./modules/vpc"
}

output "bucket_name" {
  value = aws_s3_bucket.logs.id
}

locals {
  env = "prod"
}
