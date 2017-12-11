#!/bin/bash

# GC settings
# project
PROJECT=$(cat settings | grep PROJECT= | head -1 | cut -f2 -d"=")

# zone
ZONE=$(cat settings | grep ZONE= | head -1 | cut -f2 -d"=")
#

# GKE cluster VM name
SERVERS=$(cat settings | grep SERVERS= | head -1 | cut -f2 -d"=")

# static IP for the internal LB VM
STATIC_IP=$(cat settings | grep STATIC_IP= | head -1 | cut -f2 -d"=")

# VM type
MACHINE_TYPE=$(cat settings | grep MACHINE_TYPE= | head -1 | cut -f2 -d"=")

# VMs name
BASE_VM_NAME=$SERVERS-lb-base
VM_NAME=$SERVERS-int-lb

#
gcloud config set project $PROJECT
gcloud config set compute/zone $ZONE

# clean up the old setup if such exists
# Delete VM
echo "Deleting the old VM if such exists ..."
VM_FULL_NAME=$(gcloud compute instances list | grep -v grep | grep $VM_NAME | awk {'print $1'})
yes | gcloud compute instances delete $VM_FULL_NAME
echo " "

# Delete the instance group
echo "Deleting the old instance group if such exists ..."
yes | gcloud compute instance-groups managed delete $VM_NAME
echo " "

# delete the instance template
echo "Deleting the old instance template if such exists ..."
yes | gcloud compute instance-templates delete $VM_NAME-template
echo " "

# delete the base image
echo "Deleting the base image if such exists ..."
yes | gcloud compute images delete $BASE_VM_NAME-image
echo " "

# delete the boot disk
echo "Deleting $BASE_VM_NAME boot disk if such exists ..."
yes | gcloud compute disks delete $BASE_VM_NAME
echo " "

# Delete the old route
echo "Deleting the old route if such exists ..."
OLD_ROUTE=$(gcloud compute routes list | grep $STATIC_IP | awk {'print $1'})
yes | gcloud compute routes delete $OLD_ROUTE
echo " "
#

### base VM
# create an instance which disk will be used as a base image later one
echo "Creating base VM $BASE_VM_NAME ..."
gcloud compute instances create $BASE_VM_NAME --image debian-8 \
 --scopes compute-rw --machine-type=$MACHINE_TYPE --can-ip-forward
echo " "

echo "Waiting for VM $BASE_VM_NAME to be ready..."
VM_EXT_IP=$(gcloud compute instances list | grep -v grep | grep $BASE_VM_NAME | awk {'print $5'})
ssh -q $VM_EXT_IP exit
echo " "

# install haproxy
echo "Installing haproxy ..."
gcloud compute ssh $BASE_VM_NAME --command "sudo apt-get update && sudo apt-get -y install haproxy"
echo " "

# make a folder /opt/haproxy to store the config file
gcloud compute ssh $BASE_VM_NAME --command "sudo mkdir -p /opt/haproxy  && sudo cp -f /etc/haproxy/haproxy.cfg /opt/haproxy/haproxy.cfg.template"

# update haproxy.cfg.template file
echo "Updating haproxy config template ..."
gcloud compute ssh $BASE_VM_NAME --command \
'echo -e "\n\n# Listen for incoming traffic
listen http-lb *:80
    mode http
    balance roundrobin
    option httpclose
    option forwardfor" | sudo tee -a /opt/haproxy/haproxy.cfg.template'
echo " "

# update and copy 'get_vms_ip' script to the VM
echo "Copying get_vm_ip script to VM $BASE_VM_NAME ..."
cp -f get_vms_ip.tmpl get_vms_ip
sed -i "" 's/_PROJECT_/'$PROJECT'/' get_vms_ip
sed -i "" 's/_SERVERS_/'$SERVERS'/' get_vms_ip
gcloud compute copy-files get_vms_ip $BASE_VM_NAME:/tmp
gcloud compute ssh $BASE_VM_NAME --command "sudo mkdir -p /opt/bin && sudo cp /tmp/get_vms_ip /opt/bin && sudo chmod +x /opt/bin/get_vms_ip"
echo " "

# enable cron job to run each 2 minutes
echo "Enabling cron job for get_vm_ip script ..."
gcloud compute ssh $BASE_VM_NAME --command 'echo "*/2 * * * * /opt/bin/get_vms_ip" | sudo tee /var/spool/cron/crontabs/root && sudo chmod 600 /var/spool/cron/crontabs/root'
echo " "

# add network eth0:0 with the static IP
echo "Updating  /etc/network/interfaces file ..."
gcloud compute ssh $BASE_VM_NAME --command \
'echo -e "# static IP
auto eth0:0
iface eth0:0 inet static
  address '$STATIC_IP'
  netmask 255.255.255.0" | sudo tee -a /etc/network/interfaces'
echo " "
echo "Enabling eth0:0 network interface ..."
gcloud compute ssh $BASE_VM_NAME --command "sudo ifup eth0:0"
echo " "
###

sleep 5

### Create the custom image
# Terminate the instance but keep the boot disk
echo "Stoping base VM $BASE_VM_NAME ..."
gcloud compute instances stop $BASE_VM_NAME
echo " "
echo "Deleting base VM $BASE_VM_NAME but keeping it's boot disk ..."
yes | gcloud compute instances delete $BASE_VM_NAME --keep-disks boot
echo " "
# Create the custom image using the source disk that you just created
echo "Creating the custom image $BASE_VM_NAME-image using base VM $BASE_VM_NAME boot disk as the source ..."
gcloud compute images create $BASE_VM_NAME-image --source-disk $BASE_VM_NAME
echo " "
#

# Create an HAProxy instance template based on the custom image
echo "Creating the $VM_NAME instance template based on the $BASE_VM_NAME-image image ..."
gcloud compute instance-templates create $VM_NAME-template --image $BASE_VM_NAME-image \
 --scopes compute-rw --machine-type=$MACHINE_TYPE --can-ip-forward
echo " "

# Create a managed instance group
echo "Creating a managed instance group named $VM_NAME ..."
gcloud compute instance-groups managed create $VM_NAME \
 --base-instance-name $VM_NAME --size 1 --template $VM_NAME-template
echo " "

sleep 25

### Set static IP route

# Get VM's full name
VM_FULL_NAME=$(gcloud compute instances list | grep -v grep | grep $VM_NAME | awk {'print $1'})

# Create the route for VM's static IP
echo "Creating the route for $VM_FULL_NAME static IP $STATIC_IP ..."
gcloud compute routes create ip-$VM_FULL_NAME \
         --next-hop-instance $VM_FULL_NAME \
            --next-hop-instance-zone $ZONE \
                --destination-range $STATIC_IP/32
echo " "
echo "The end ..."
