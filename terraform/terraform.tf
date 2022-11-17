terraform {
    required_providers {
        aws = {
            source = "hashicorp/aws"
            version = "~> 4.0"
        }
    }
}

provider "aws" {
  region = "eu-west-2"
}

data "archive_file" "lambda_functions_archive" {
    type = "zip"

    source_dir = "${path.module}/functions"
    output_path = "${path.module}/function.zip"
}

resource "aws_s3_bucket" "lambda_bucket" {

}

resource "aws_s3_object" "lambda_functions" {
    bucket = aws_s3_bucket.lambda_bucket.id

    key = "functions.zip"
    source = data.archive_file.lambda_functions_archive.output_path

    etag = filemd5(data.archive_file.lambda_functions_archive.output_path)
}

resource "aws_iam_role" "receive_url_func_role" {
    assume_role_policy = jsonencode({
        Version = "2012-10-17"
        Statement = [
            {
                Action = "sts:AssumeRole"
                Effect = "Allow"
                Sid    = ""
                Principal = {
                    Service = "lambda.amazonaws.com"
                }
            }
        ]
    })

    inline_policy {
        name = "write_queue"
        policy = jsonencode({
            Version = "2012-10-17"
            Statement = [
                {
                    Sid = ""
                    Effect = "Allow"
                    Action = "sqs:SendMessage"
                    Resource = aws_sqs_queue.image_queue.arn
                }
            ]
        })
    }
}

resource "aws_lambda_function" "receive_url_func" {
    function_name = "demo-receive-url-func"
    runtime = "nodejs16.x"

    s3_bucket = aws_s3_bucket.lambda_bucket.id
    s3_key = aws_s3_object.lambda_functions.key
    // source_code_hash = filebase64sha256("functions.zip")
    handler = "receiveImageUrl.handler"

    role = aws_iam_role.receive_url_func_role.arn

    environment {
        variables = {
          QUEUE_URL = aws_sqs_queue.image_queue.url
        }
    }
}

resource "aws_lambda_function_url" "recive_url_func_url" {
    function_name = aws_lambda_function.receive_url_func.function_name
    authorization_type = "NONE"
}

output "receive_url_func" {
    value = aws_lambda_function_url.recive_url_func_url.function_url
}

resource "aws_iam_role" "download_image_func_role" {
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Sid    = ""
      Principal = {
        Service = "lambda.amazonaws.com"
      }
      }
    ]
  })

  inline_policy {
    name = "execution-role"
    policy = jsonencode({
        Version = "2012-10-17"
        Statement = [
            {
                Sid = ""
                Effect = "Allow"
                Action = "s3:PutObject"
                Resource = "${aws_s3_bucket.images_bucket.arn}/*"
            },
            {
              Sid = ""
              Effect = "Allow"
              Action = [
                "sqs:ReceiveMessage",
                "sqs:DeleteMessage",
                "sqs:GetQueueAttributes"
              ]
              Resource = aws_sqs_queue.image_queue.arn
            }
        ]
    })
  }
}

resource "aws_lambda_function" "download_image_func" {
    function_name = "demo-download-image-func"
    runtime = "nodejs16.x"

    s3_bucket = aws_s3_bucket.lambda_bucket.id
    s3_key = aws_s3_object.lambda_functions.key
    // source_code_hash = filebase64sha256("functions.zip")
    handler = "downloadImage.handler"

    role = aws_iam_role.download_image_func_role.arn

    environment {
        variables = {
          IMAGE_BUCKET = aws_s3_bucket.images_bucket.id
        }
    }
}

resource "aws_lambda_event_source_mapping" "image_queue_lambda_trigger" {
    event_source_arn = aws_sqs_queue.image_queue.arn
    function_name = aws_lambda_function.download_image_func.arn
}

resource "aws_s3_bucket" "images_bucket" {

}

resource "aws_sqs_queue" "image_queue" {

}