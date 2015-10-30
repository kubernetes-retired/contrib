
# Kubernetes Continuous Delivery
Deployment scripts for continuous integration and\or continuous delivery of kubernetes projects. This project was tested and released using a private install of both CircleCI and Jenkins. The core deployments scripts (./deploy/) are used for both systems and as a result are designed to be extensible. Please contribute to add features and support for different CI/CD systems as needed.

## Usage

In general, the documentation for scripts is handled inline with comments. You must have a [kubernetes config](http://kubernetes.io/v1.0/docs/user-guide/kubeconfig-file.html) file available and accessible to your build system from a URL. S3 was used in testing.  See build environment setup instructions for Jenkins and CircleCI. 

You must have at least one running kubernetes cluster. If you intend to deploy to production install multiple kubernetes clusters and run the deploy command multiple times with the different context names from your kube config file.

Deployment scripts are in the ./deploy/ folder.
..* ./deploy/ensure-kubectl.sh - pulls down the kubectl binary if it doesn't exist and installs packages that are expected to be in place if missing.
..* ./deploy/deploy-service.sh - call kubectl commands to deploy services based on the yaml inside your project.


## Jenkins
If you already have a Jenkins instance running with a local docker daemon installed on the builder box, you should be able to get going by doing the following.

1. Create your jenkins job and link it to your github account using the git source code management plugin.
2. Create credentials for your docker registry in Jenkins.
3. Map the credentials to the dockeruser and dockerpass environment variables.
4. Create an execute shell command as follows:
```
cd $WORKSPACE
chmod +x ./jenkins.sh && ./jenkins.sh
```
5. Update the environment variables in the jenkins.sh
6. push changes to github and check the Jenkins job console output for errors\success messages.

## Circle CI
