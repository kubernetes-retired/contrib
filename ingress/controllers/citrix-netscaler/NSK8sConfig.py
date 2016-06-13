#!/usr/bin/env python

#
# Copyright (c) 2008-2016 Citrix Systems, Inc.
#
#   Licensed under the Apache License, Version 2.0 (the "License")
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http:#www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
#

#
# This script will run on the Kube-master node. It provides the functionality of
# adding and removing vxlan and MAC configuration for the NetScaler to
# interoperate in the Kubernetes cluster.
# abhishek <dot> dhamija <at> citrix <dot> com
#

#
# Examples:
# VXLAN add : python NSK8sConfig.py addvxlan 10.217.129.75 nsroot nsroot 10.11.50.13 10.11.50.10
# VXLAN rem : python NSK8sConfig.py addvxlan 10.217.129.75 nsroot nsroot 10.11.50.13 10.11.50.10
# MAC add   : python NSK8sConfig.py addmac 10.11.50.10 10.11.50.13 d2:15:53:cd:46:60 10.254.51.0 24
# MAC rem   : python NSK8sConfig.py addmac 10.11.50.10 10.11.50.13 d2:15:53:cd:46:60 10.254.51.0 24
#

import os
import sys
import subprocess
from nssrc.com.citrix.netscaler.nitro.exception.nitro_exception import nitro_exception
from nssrc.com.citrix.netscaler.nitro.resource.config.network.vxlan import vxlan
from nssrc.com.citrix.netscaler.nitro.resource.config.network.iptunnel import iptunnel
from nssrc.com.citrix.netscaler.nitro.resource.config.network.vxlan_iptunnel_binding import vxlan_iptunnel_binding
from nssrc.com.citrix.netscaler.nitro.service.nitro_service import nitro_service

DEVNULL = open(os.devnull, 'wb')

class NSK8sConfig :

################################################################################
# __init__
#
  def __init__(self):
    _operation=""
    _nsip=""
    _username=""
    _password=""
    _nsepip=""
    _kmasterip=""

################################################################################
# Helper routines
#
  @staticmethod
  def NSlogin(nsip, username, password):
    # Create an instance of the nitro_service class to connect to the appliance
    ns_session = nitro_service(nsip,"HTTP")
    ns_session.set_credential(username, password)
    ns_session.timeout = 310

    # Log on to the appliance using your credentials
    ns_session.login()
    return ns_session


  @staticmethod
  def NSlogout(ns_session):
    # Save the configurations
    ns_session.save_config()

    # Logout from the NetScaler appliance
    ns_session.logout()

  @staticmethod
  def usage():
    print("Usage: NSK8sConfig.py <operation addvxlan|remvxlan> <nsip> <username> <password> <nsepip> <kmasterip>")
    print("       NSK8sConfig.py <operation addmac|remmac> <kmasterip> <nsepip> <nsmac> <nsflannelsubnet> <nsflannelsubnetmask[0-32]>")

################################################################################
# Addmac subroutine
#
  @staticmethod
  def addmac(cls, args_):
    config = NSK8sConfig()
    config.operation  = args_[1]
    config.kmasterip  = args_[2]
    config.nsepip     = args_[3]
    config.nsmac      = args_[4]
    config.nsFsubnet  = args_[5]
    config.nsFmask    = args_[6]

    # Construct the command
    command = 'etcdctl --no-sync --peers http://%s:4001 mk ' \
              '/flannel/network/subnets/%s-%s \"{\\\"PublicIP\\\":\\\"%s\\\",' \
              '\\\"BackendType\\\":\\\"vxlan\\\",\\\"BackendData\\\":' \
              '{\\\"VtepMAC\\\":\\\"%s\\\"}}\"' % (config.kmasterip, \
              config.nsFsubnet, config.nsFmask, config.nsepip, config.nsmac)
    subprocess.check_output(command, stderr=DEVNULL, shell=True)

################################################################################
# Remmac subroutine
#
  @staticmethod
  def remmac(cls, args_):
    config = NSK8sConfig()
    config.operation  = args_[1]
    config.kmasterip  = args_[2]
    config.nsepip     = args_[3]
    config.nsmac      = args_[4]
    config.nsFsubnet  = args_[5]
    config.nsFmask    = args_[6]

    # Construct the command
    command = 'etcdctl --no-sync --peers http://%s:4001 rm ' \
              '/flannel/network/subnets/%s-%s' % (config.kmasterip, \
              config.nsFsubnet, config.nsFmask)
    subprocess.check_output(command, stderr=DEVNULL, shell=True)

################################################################################
# Addvxlan subroutine
#
  @staticmethod
  def addvxlan(cls, args_):
    config = NSK8sConfig()
    config.operation  = args_[1]
    config.nsip       = args_[2]
    config.username   = args_[3]
    config.password   = args_[4]
    config.nsepip     = args_[5]
    config.kmasterip  = args_[6]

    # Identify the VXLAN id
    command = 'etcdctl --debug --no-sync --peers http://%s:4001 get /flannel/network/config | grep VNI | awk \'{print $NF}\'' % config.kmasterip
    vni = subprocess.check_output(command, stderr=DEVNULL, shell=True).rstrip()

    # Indentify the Kube-minion node IPs
    command = 'kubectl -s http://%s:8080 describe nodes | grep Addresses: | awk -F, \'{print $NF}\'' % config.kmasterip
    kminions = subprocess.check_output(command, stderr=DEVNULL, shell=True).split()

    try :

      # Login to Netscaler
      ns_session = NSK8sConfig().NSlogin(config.nsip, config.username, config.password)

      # Create an instance of the virtual server class
      new_vxlan_obj = vxlan()

      # Create a new vxlan
      new_vxlan_obj.id    = str(vni)
      new_vxlan_obj.port  = "8472"  # Known port for VXLAN communication
      vxlan.add(ns_session, new_vxlan_obj)

      # Create iptunnel and bind it to vxlan for each minion node
      for minion in kminions :
        new_iptunnel_obj = iptunnel()
        new_iptunnel_obj.name     = 'tunnel-%s' % minion
        new_iptunnel_obj.remote   = minion
        new_iptunnel_obj.remotesubnetmask = "255.255.255.255" # Expected value
        new_iptunnel_obj.local    = config.nsepip
        new_iptunnel_obj.protocol = "VXLAN"
        iptunnel.add(ns_session, new_iptunnel_obj)

        new_vxlan_iptunnel_binding = vxlan_iptunnel_binding()
        new_vxlan_iptunnel_binding.tunnel = 'tunnel-%s' % minion
        new_vxlan_iptunnel_binding.id     = str(vni)
        vxlan_iptunnel_binding.add(ns_session, new_vxlan_iptunnel_binding)

      # Logout from Netscaler
      NSK8sConfig().NSlogout(ns_session)

    except nitro_exception as  e:
      print("Exception::errorcode="+str(e.errorcode)+",message="+ e.message)
    except Exception as e:
      print("Exception::message="+str(e.args))
    return

################################################################################
# Remvxlan subroutine
#
  @staticmethod
  def remvxlan(cls, args_):
    config = NSK8sConfig()
    config.operation  = args_[1]
    config.nsip       = args_[2]
    config.username   = args_[3]
    config.password   = args_[4]
    config.nsepip     = args_[5]
    config.kmasterip  = args_[6]

    # Identify the VXLAN id
    command = 'etcdctl --no-sync --peers http://%s:4001 get /flannel/network/config | grep VNI | awk \'{print $NF}\'' % config.kmasterip
    vni = subprocess.check_output(command, stderr=DEVNULL, shell=True).rstrip()

    # Indentify the Kube-minion node IPs
    command = 'kubectl -s http://%s:8080 describe nodes | grep Addresses: | awk -F, \'{print $NF}\'' % config.kmasterip
    kminions = subprocess.check_output(command, stderr=DEVNULL, shell=True).split()

    try :

      # Login to Netscaler
      ns_session = NSK8sConfig().NSlogin(config.nsip, config.username, config.password)

      # Delete vxlan
      new_vxlan_obj = vxlan()
      new_vxlan_obj.id    = str(vni)
      vxlan.delete(ns_session, new_vxlan_obj)

      # Delete iptunnel for each minion
      for minion in kminions :
        new_iptunnel_obj = iptunnel()
        new_iptunnel_obj.name     = 'tunnel-%s' % minion
        iptunnel.delete(ns_session, new_iptunnel_obj)

      # Logout from Netscaler
      NSK8sConfig().NSlogout(ns_session)

    except nitro_exception as  e:
      print("Exception::errorcode="+str(e.errorcode)+",message="+ e.message)
    except Exception as e:
      print("Exception::message="+str(e.args))
    return


################################################################################
# main subroutine
#
  @staticmethod
  def main(cls, args_):
    if(len(args_) < 7):
      NSK8sConfig().usage()
      return

    config = NSK8sConfig()
    config.operation  = args_[1]

    if config.operation == "addvxlan" :
      NSK8sConfig().addvxlan(NSK8sConfig(), args_)
    elif config.operation == "remvxlan" :
      NSK8sConfig().remvxlan(NSK8sConfig(), args_)
    elif config.operation == "addmac" :
      NSK8sConfig().addmac(NSK8sConfig(), args_)
    elif config.operation == "remmac" :
      NSK8sConfig().remmac(NSK8sConfig(), args_)
    else :
      NSK8sConfig().usage()
      return

################################################################################
# Main thread of execution
#
if __name__ == '__main__':
  try:
    if len(sys.argv) != 7:
      sys.exit()
    else:
      NSK8sConfig().main(NSK8sConfig(),sys.argv)
  except SystemExit:
    NSK8sConfig().usage()

################################################################################
