const AWS = require('aws-sdk');
const https = require('https');
const s3 = new AWS.S3();

const getThenUpload = (url, callback) => {
  https.get(url, (res) => {
    const data = [];

    res.on('data', (chunk) => {
      data.push(chunk);
    });

    res.on('end', () => {
      const params = {
        Bucket: process.env.IMAGE_BUCKET,
        Key: `${new Date().toISOString()}.jpg`,
        Body: Buffer.concat(data),
        ContentType: 'image/jpg',
      };

      s3.upload(params, (err, rsp) => {
        if (err) {
          console.error(err, err.stack);
          callback(err, { statusCode: 404, err });
        } else {
          console.log(rsp);
          callback(null, { statusCode: 200 });
        }
      });
    });
  });
};

exports.handler = function (event, context) {
    const message = event.Records[0];
    const payload = JSON.parse(message.body);
    const url = payload['url'];
    
    getThenUpload(url, (err, data) => {
      if (err) {
        console.error(`Error: ${err}`);
      } else {
        console.log(`Data: ${JSON.stringify(data)}`);
      }
      
       context.done(null, '');
    });
};