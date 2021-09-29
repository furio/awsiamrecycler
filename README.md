AWS IAM Recycler
==============

A toy project to study Kubernetes operator programming.

The operator is able to recycle IAM access and secret key of an user inside a secret.

This is made via `operator sdk` so with a `make docker-build deploy` you can build it and deploy to default context present in your `~/.kube/config` file.

The basic mode of operation is implemented and consist of an object that needs to know a destination secret object where to edit the access/secret keys, the iam user and, the number of minutes to wait between each recycling.

The operator itself needs to run with a set of IAM credentials that are linked to a IAM permission like:
```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "iam:DeleteAccessKey",
                "iam:CreateAccessKey",
                "iam:ListAccessKeys"
            ],
            "Resource": "*",
            "Condition": {
                "StringEquals": {
                    "iam:ResourceTag/controlled-by": "kube"
                }
            }
        }
    ]
}
```

The policy contains a `Condition` that limit the operator ability to edit IAM users that has that tag (hopefully).

Todo
----
- Understand and write Tests for an Operator
- Bundling for installation
- Automation via Github Actions

Example
--------
This is an example of the `IAMRecycler` object and how it works
```
apiVersion: v1
kind: Namespace
metadata:
  name: awstest-ns
---
apiVersion: aws.furio.me/v1alpha1
kind: IAMRecycler
metadata:
  name: iamrecycler-test-01
  namespace: awstest-ns
spec:
  secret: secret-change
  iamuser: test-change-keys
  recycle: 5
  datakeyaccesskey: AWS_ACCESS_KEY_ID
  datakeysecretkey: AWS_SECRET_ACCESS_KEY
---
apiVersion: v1
kind: Secret
metadata:
  name: secret-change
  namespace: awstest-ns
type: Opaque
data:
  AWS_ACCESS_KEY_ID: ""
  AWS_SECRET_ACCESS_KEY: ""
  AWS_DEFAULT_REGION: ZXUtd2VzdC0x
```

The object needs to know the secret object name, the data "keys" to edit, the iam user and, the number of minutes to wait between each recycling.