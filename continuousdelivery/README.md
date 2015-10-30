
# testDeployment
Deployment scripts for continuous integration and continuous delivery. This projected is tested and released using a private install of both CircleCI and Jenkins. 

## Usage

See instructions for Jenkins and CircleCI.

Deployment scripts are in the deploy folder.
./deploy/ensure-kubectl.sh - pulls down the kubectl binary if it doesn't exist and installs packages that are expected to be in place if missing.
./deploy/deploy-service.sh - call kubectl commands to deploy services based on the yaml inside your project.



