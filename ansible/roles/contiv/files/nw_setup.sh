#!/bin/bash

sleep 30
#create default nw and epg
/usr/bin/contivctl network create -public no -encap vxlan -subnet 20.1.1.0/24 -gateway 20.1.1.254 default-net
/usr/bin/contivctl group create default-net default-epg


#create poc nw and epg
/usr/bin/contivctl network create -public no -encap vxlan -subnet 21.1.1.0/24 -gateway 21.1.1.254 poc-net
/usr/bin/contivctl group create poc-net poc-epg
