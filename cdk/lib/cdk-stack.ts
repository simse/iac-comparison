import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as sqs from 'aws-cdk-lib/aws-sqs';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as s3 from 'aws-cdk-lib/aws-s3';
import { SqsEventSource } from 'aws-cdk-lib/aws-lambda-event-sources';

export class CdkStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const imageQueue = new sqs.Queue(this, 'ImageDownloadQueue', {
      visibilityTimeout: cdk.Duration.seconds(300)
    });

    const receiveImageUrlFunc = new lambda.Function(this, 'receiveImageUrlFunc', {
      runtime: lambda.Runtime.NODEJS_16_X,
      handler: 'receiveImageUrl.handler',
      code: lambda.Code.fromAsset("functions"),
      environment: {
        QUEUE_URL: imageQueue.queueUrl
      }
    });

    const receiveImageUrlFuncUrl = receiveImageUrlFunc.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE
    });

    receiveImageUrlFunc.addToRolePolicy(
      new iam.PolicyStatement({
        actions: ['sqs:SendMessage'],
        resources: [imageQueue.queueArn]
      })
    );

    new cdk.CfnOutput(this, 'receiveImageUrl', {
      value: receiveImageUrlFuncUrl.url
    });

    const imageBucket = new s3.Bucket(this, 'imagesBucket');

    const downloadImageFunc = new lambda.Function(this, 'downloadImageFunc', {
      runtime: lambda.Runtime.NODEJS_16_X,
      handler: 'downloadImage.handler',
      code: lambda.Code.fromAsset("functions"),
      environment: {
        IMAGE_BUCKET: imageBucket.bucketName
      }
    });

    downloadImageFunc.addToRolePolicy(
      new iam.PolicyStatement({
        actions: ['s3:PutObject'],
        resources: [imageBucket.bucketArn+'/*']
      })
    );

    downloadImageFunc.addToRolePolicy(
      new iam.PolicyStatement({
        actions: ['sqs:ReceiveMessage', 'sqs:DeleteMessage', 'sqs:GetQueueAttributes'],
        resources: [imageQueue.queueArn]
      })
    );

    downloadImageFunc.addEventSource(new SqsEventSource(imageQueue));
  }
}
