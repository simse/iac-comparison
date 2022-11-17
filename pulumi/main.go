package main

import (
	"encoding/json"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/sqs"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create an AWS resource (S3 Bucket)
		bucket, err := s3.NewBucket(ctx, "images-bucket", nil)
		if err != nil {
			return err
		}

		imageQueue, err := sqs.NewQueue(ctx, "image-download-queue", &sqs.QueueArgs{})
		if err != nil {
			return err
		}

		receiveImageUrlRole, err := iam.NewRole(ctx, "receiveImageUrlRole", &iam.RoleArgs{
			AssumeRolePolicy: pulumi.Any(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
`)})
		if err != nil {
			return err
		}

		sendQueueMessagePolicy := imageQueue.Arn.ApplyT(func(arn string) (string, error) {
			policyJSON, err := json.Marshal(map[string]interface{}{
				"Version": "2012-10-17",
				"Statement": []interface{}{
					map[string]interface{}{
						"Action": []string{
							"sqs:SendMessage",
						},
						"Effect": "Allow",
						"Resource": []string{
							arn,
						},
					},
				},
			})
			if err != nil {
				return "", err
			}
			return string(policyJSON), nil
		})

		policy, err := iam.NewPolicy(ctx, "iam-policy", &iam.PolicyArgs{
			Policy: sendQueueMessagePolicy,
		})
		if err != nil {
			return err
		}

		_, err = iam.NewRolePolicyAttachment(ctx, "sendQueueMessagePolicyAttachment", &iam.RolePolicyAttachmentArgs{
			Role:      receiveImageUrlRole.Name,
			PolicyArn: policy.Arn,
		})
		if err != nil {
			return err
		}

		receiveUrlFunc, err := lambda.NewFunction(ctx, "receiveUrl", &lambda.FunctionArgs{
			Code:    pulumi.NewFileArchive("functions.zip"),
			Role:    receiveImageUrlRole.Arn,
			Handler: pulumi.String("receiveImageUrl.handler"),
			Runtime: pulumi.String("nodejs16.x"),
			Environment: &lambda.FunctionEnvironmentArgs{
				Variables: pulumi.StringMap{
					"QUEUE_URL": imageQueue.Url,
				},
			},
		})
		if err != nil {
			return err
		}

		receiveUrlFuncUrl, err := lambda.NewFunctionUrl(ctx, "receiveUrlFuncUrl", &lambda.FunctionUrlArgs{
			FunctionName:      receiveUrlFunc.Name,
			AuthorizationType: pulumi.String("NONE"),
		})
		if err != nil {
			return err
		}
		ctx.Export("receiveUrlFunc", receiveUrlFuncUrl.FunctionUrl)

		downloadImageFuncRole, err := iam.NewRole(ctx, "downloadImageFunc", &iam.RoleArgs{
			AssumeRolePolicy: pulumi.Any(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
`)})
		if err != nil {
			return err
		}

		receiveQueueMessagePolicyContents := imageQueue.Arn.ApplyT(func(arn string) (string, error) {
			policyJSON, err := json.Marshal(map[string]interface{}{
				"Version": "2012-10-17",
				"Statement": []interface{}{
					map[string]interface{}{
						"Action": []string{
							"sqs:ReceiveMessage",
							"sqs:DeleteMessage",
							"sqs:GetQueueAttributes",
						},
						"Effect": "Allow",
						"Resource": []string{
							arn,
						},
					},
				},
			})
			if err != nil {
				return "", err
			}
			return string(policyJSON), nil
		})

		receiveQueueMessagePolicy, err := iam.NewPolicy(ctx, "receive-queue-message-policy", &iam.PolicyArgs{
			Policy: receiveQueueMessagePolicyContents,
		})
		if err != nil {
			return err
		}

		writeBucketObjectContents := bucket.Arn.ApplyT(func(arn string) (string, error) {
			policyJSON, err := json.Marshal(map[string]interface{}{
				"Version": "2012-10-17",
				"Statement": []interface{}{
					map[string]interface{}{
						"Action": []string{
							"s3:PutObject",
						},
						"Effect": "Allow",
						"Resource": []string{
							arn + "/*",
						},
					},
				},
			})
			if err != nil {
				return "", err
			}
			return string(policyJSON), nil
		})

		writeBucketObject, err := iam.NewPolicy(ctx, "write-image-policy", &iam.PolicyArgs{
			Policy: writeBucketObjectContents,
		})
		if err != nil {
			return err
		}

		_, err = iam.NewRolePolicyAttachment(ctx, "receiveQueueMessageAttachment", &iam.RolePolicyAttachmentArgs{
			Role:      downloadImageFuncRole.Name,
			PolicyArn: receiveQueueMessagePolicy.Arn,
		})
		if err != nil {
			return err
		}

		_, err = iam.NewRolePolicyAttachment(ctx, "writeImageAttachment", &iam.RolePolicyAttachmentArgs{
			Role:      downloadImageFuncRole.Name,
			PolicyArn: writeBucketObject.Arn,
		})
		if err != nil {
			return err
		}

		downloadImageFunc, err := lambda.NewFunction(ctx, "downloadImageFunc", &lambda.FunctionArgs{
			Code:    pulumi.NewFileArchive("functions.zip"),
			Role:    downloadImageFuncRole.Arn,
			Handler: pulumi.String("downloadImage.handler"),
			Runtime: pulumi.String("nodejs16.x"),
			Environment: &lambda.FunctionEnvironmentArgs{
				Variables: pulumi.StringMap{
					"IMAGE_BUCKET": bucket.Bucket,
				},
			},
		})
		if err != nil {
			return err
		}

		_, err = lambda.NewEventSourceMapping(ctx, "imageQueueDownloadTrigger", &lambda.EventSourceMappingArgs{
			EventSourceArn: imageQueue.Arn,
			FunctionName:   downloadImageFunc.Arn,
		})
		if err != nil {
			return err
		}

		// Export the name of the bucket
		ctx.Export("bucketName", bucket.ID())
		return nil
	})
}
