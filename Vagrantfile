# -*- mode: ruby -*-
# vi: set ft=ruby :

# This Vagrantfile provides a simple default configuration using VirtualBox.
# For any other configuration, create a configuration in .vagrant-openshift.json
# using the vagrant-openshift plugin (https://github.com/openshift/vagrant-openshift)
# as an alternative to editing this file.
# Specific providers may use further configuration from provider-specific files -
# consult the provider definitions below for specifics.

# Vagrantfile API/syntax version. Don't touch unless you know what you're doing!
VAGRANTFILE_API_VERSION = "2"

# Require a recent version of vagrant otherwise some have reported errors setting host names on boxes
Vagrant.require_version ">= 1.6.2"

def pre_vagrant_171
  @pre_vagrant_171 ||= begin
    req = Gem::Requirement.new("< 1.7.1")
    if req.satisfied_by?(Gem::Version.new(Vagrant::VERSION))
      true
    else
      false
    end
  end
end

def full_provision(vm, username = [])
  if pre_vagrant_171
    vm.provision "shell", path: "vagrant/provision-full.sh", args: username, id: "setup"
  else
    vm.provision "setup", type: "shell", path: "vagrant/provision-full.sh", args: username
  end
end

# @param tgt [Hash] target hash that we will be **altering**
# @param src [Hash] read from this source hash
# @return the modified target hash
# @note this one does not merge Array elements
def hash_deep_merge!(tgt_hash, src_hash)
  tgt_hash.merge!(src_hash) { |key, oldval, newval|
    if oldval.kind_of?(Hash) && newval.kind_of?(Hash)
      hash_deep_merge!(oldval, newval)
    else
      newval
    end
  }
end

class VFLoadError < Vagrant::Errors::VagrantError
  def error_message; @parserr; end
  def initialize(message, *args)
    @parserr = message
    super(*args)
  end
end
OPENSTACK_CRED_FILE = "~/.openstackcred"
OPENSTACK_BOX_URL   = "https://github.com/cloudbau/vagrant-openstack-plugin/raw/master/dummy.box"
AWS_CRED_FILE       = "~/.awscred"
AWS_BOX_URL         = "https://github.com/mitchellh/vagrant-aws/raw/master/dummy.box"
VM_NAME_PREFIX      = ENV['OPENSHIFT_VM_NAME_PREFIX'] || ""

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|

  # these are the default settings, overrides are in .vagrant-openshift.json
  vagrant_openshift_config = {
    "instance_name"     => "origin-dev",
    "os"                => "fedora",
    "dev_cluster"       => false,
    "dind_dev_cluster"  => ENV['OPENSHIFT_DIND_DEV_CLUSTER'] || false,
    "insert_key"        => true,
    "num_minions"       => ENV['OPENSHIFT_NUM_MINIONS'] || 2,
    "rebuild_yum_cache" => false,
    "cpus"              => ENV['OPENSHIFT_NUM_CPUS'] || 2,
    "memory"            => ENV['OPENSHIFT_MEMORY'] || 2048,
    "sync_folders_type" => nil,
    "master_ip"         => ENV['OPENSHIFT_MASTER_IP'] || "10.245.2.2",
    "minion_ip_base"    => ENV['OPENSHIFT_MINION_IP_BASE'] || "10.245.2.",
    "virtualbox"        => {
      "box_name" => "fedora_inst",
      "box_url" => "https://mirror.openshift.com/pub/vagrant/boxes/openshift3/fedora_virtualbox_inst.box"
    },
    "vmware"            => {
      "box_name" => "fedora_inst",
      "box_url"  => "http://opscode-vm-bento.s3.amazonaws.com/vagrant/vmware/opscode_fedora-21_chef-provisionerless.box"
    },
    "libvirt"           => {
      "box_name" => "fedora_inst",
      "box_url"  => "https://mirror.openshift.com/pub/vagrant/boxes/openshift3/fedora_libvirt_inst.box"
    },
    "aws"               => {
      "_see_also_"   => AWS_CRED_FILE,
      "box_name"     => "aws-dummy-box",
      "box_url"      => AWS_BOX_URL,
      "ami"          => "<AMI>",
      "ami_region"   => "<AMI_REGION>",
      "ssh_user"     => "<SSH_USER>"
    },
    "openstack" => {
      '_see_also_'  => OPENSTACK_CRED_FILE,
      'box_name'    => "openstack-dummy-box",
      'box_url'     => OPENSTACK_BOX_URL,
      'image'       => "Fedora",
      'ssh_user'    => "root"
    },
  }

  # attempt to read config in this repo's .vagrant-openshift.json if present
  if File.exist?('.vagrant-openshift.json')
    json = File.read('.vagrant-openshift.json')
    begin
      hash_deep_merge!(vagrant_openshift_config, JSON.parse(json))
    rescue JSON::ParserError => e
      raise VFLoadError.new "Error parsing .vagrant-openshift.json:\n#{e}"
    end
  end

  # Determine the OS platform to use
  kube_os = vagrant_openshift_config['os'] || "fedora"

  # OS platform to box information
  kube_box = {
    "fedora" => {
      "name" => "fedora_deps",
      "box_url" => "https://mirror.openshift.com/pub/vagrant/boxes/openshift3/fedora_virtualbox_deps.box"
    }
  }

  dind_dev_cluster = vagrant_openshift_config['dind_dev_cluster']
  dev_cluster = vagrant_openshift_config['dev_cluster'] || ENV['OPENSHIFT_DEV_CLUSTER']
  if dind_dev_cluster
    config.vm.define "#{VM_NAME_PREFIX}dind-host" do |config|
      config.vm.box = kube_box[kube_os]["name"]
      config.vm.box_url = kube_box[kube_os]["box_url"]
      config.vm.provision "shell", inline: "/vagrant/vagrant/provision-full.sh"
      config.vm.provision "shell", inline: "/vagrant/hack/dind-cluster.sh start"
      config.vm.hostname = "openshift-dind-host"
      config.vm.synced_folder ".", "/vagrant", type: vagrant_openshift_config['sync_folders_type']
    end
  elsif dev_cluster
    # Start an OpenShift cluster
    # Currently this only works with the (default) VirtualBox provider.

    instance_prefix = "openshift"

    # The number of minions to provision.
    num_minion = (vagrant_openshift_config['num_minions'] || ENV['OPENSHIFT_NUM_MINIONS'] || 2).to_i

    # IP configuration
    master_ip = vagrant_openshift_config['master_ip']
    minion_ip_base = vagrant_openshift_config['minion_ip_base']
    minion_ips = num_minion.times.collect { |n| minion_ip_base + "#{n+3}" }
    minion_ips_str = minion_ips.join(",")

    # OpenShift master
    config.vm.define "#{VM_NAME_PREFIX}master" do |config|
      config.vm.box = kube_box[kube_os]["name"]
      config.vm.box_url = kube_box[kube_os]["box_url"]
      config.vm.provision "shell", inline: "/vagrant/vagrant/provision-master.sh #{master_ip} #{num_minion} #{minion_ips_str} #{instance_prefix} #{ENV['OPENSHIFT_SDN']}"
      config.vm.network "private_network", ip: "#{master_ip}"
      config.vm.hostname = "openshift-master"
    end

    # OpenShift minion
    num_minion.times do |n|
      config.vm.define "#{VM_NAME_PREFIX}minion-#{n+1}" do |minion|
        minion_index = n+1
        minion_ip = minion_ips[n]
        minion.vm.box = kube_box[kube_os]["name"]
        minion.vm.box_url = kube_box[kube_os]["box_url"]
        minion.vm.provision "shell", inline: "/vagrant/vagrant/provision-minion.sh #{master_ip} #{num_minion} #{minion_ips_str} #{instance_prefix} #{minion_ip} #{minion_index}"
        minion.vm.network "private_network", ip: "#{minion_ip}"
        minion.vm.hostname = "openshift-minion-#{minion_index}"
      end
    end
  else # Single VM dev environment
    sync_from = vagrant_openshift_config['sync_from'] || ENV["VAGRANT_SYNC_FROM"] || '.'
    sync_to = vagrant_openshift_config['sync_to'] || ENV["VAGRANT_SYNC_TO"] || "/data/src/github.com/openshift/origin"

    ##########################
    # define settings for the single VM being created.
    config.vm.define "#{VM_NAME_PREFIX}openshiftdev", primary: true do |config|
      if vagrant_openshift_config['rebuild_yum_cache']
        config.vm.provision "shell", inline: "yum clean all && yum makecache"
      end
      if pre_vagrant_171
        config.vm.provision "shell", path: "vagrant/provision-minimal.sh", id: "setup"
      else
        config.vm.provision "setup", type: "shell", path: "vagrant/provision-minimal.sh"
      end

      config.vm.synced_folder ".", "/vagrant", disabled: true
      unless vagrant_openshift_config['no_synced_folders']
        config.vm.synced_folder sync_from, sync_to, 
          rsync__args: %w(--verbose --archive --delete), 
          type: vagrant_openshift_config['sync_folders_type'],
          nfs_udp: false # has issues when using NFS from within a docker container
      end

      if vagrant_openshift_config['private_network_ip']
        config.vm.network "private_network", ip: vagrant_openshift_config['private_network_ip']
      else
        config.vm.network "forwarded_port", guest: 80, host: 1080
        config.vm.network "forwarded_port", guest: 443, host: 1443
        config.vm.network "forwarded_port", guest: 8080, host: 8080
        config.vm.network "forwarded_port", guest: 8443, host: 8443
      end
    end

  end # vm definition(s)

  # #########################################
  # provider-specific settings defined below:

    # ################################
    # Set VirtualBox provider settings
    config.vm.provider "virtualbox" do |v, override|
      override.vm.box     = vagrant_openshift_config['virtualbox']['box_name'] unless dev_cluster
      override.vm.box_url = vagrant_openshift_config['virtualbox']['box_url'] unless dev_cluster
      override.ssh.insert_key = vagrant_openshift_config['insert_key']

      v.memory            = vagrant_openshift_config['memory'].to_i
      v.cpus              = vagrant_openshift_config['cpus'].to_i
      v.customize ["modifyvm", :id, "--cpus", vagrant_openshift_config['cpus'].to_s]
      # to make the ha-proxy reachable from the host, you need to add a port forwarding rule from 1080 to 80, which
      # requires root privilege. Use iptables on linux based or ipfw on BSD based OS:
      # sudo iptables -t nat -A PREROUTING -p tcp --dport 80 -j REDIRECT --to-port 1080
      # sudo ipfw add 100 fwd 127.0.0.1,1080 tcp from any to any 80 in
    end if vagrant_openshift_config['virtualbox']

    # ################################
    # Set libvirt provider settings
    config.vm.provider "libvirt" do |libvirt, override|
      override.vm.box     = vagrant_openshift_config['libvirt']['box_name']
      override.vm.box_url = vagrant_openshift_config['libvirt']['box_url']
      override.ssh.insert_key = vagrant_openshift_config['insert_key']
      libvirt.driver      = 'kvm'
      libvirt.memory      = vagrant_openshift_config['memory'].to_i
      libvirt.cpus        = vagrant_openshift_config['cpus'].to_i
      # run on libvirt somewhere other than default:
      libvirt.uri         = ENV["VAGRANT_LIBVIRT_URI"] if ENV["VAGRANT_LIBVIRT_URI"]
      full_provision(override.vm)
    end if vagrant_openshift_config['libvirt']

    # ###################################
    # Set VMware Fusion provider settings
    config.vm.provider "vmware_fusion" do |v, override|
      override.vm.box     = vagrant_openshift_config['vmware']['box_name']
      override.vm.box_url = vagrant_openshift_config['vmware']['box_url']
      override.ssh.insert_key = vagrant_openshift_config['insert_key']
      v.vmx["memsize"]    = vagrant_openshift_config['memory'].to_s
      v.vmx["numvcpus"]   = vagrant_openshift_config['cpus'].to_s
      v.gui               = false
      full_provision(override.vm)
    end if vagrant_openshift_config['vmware']

    # ###############################
    # Set OpenStack provider settings
    config.vm.provider "openstack" do |os, override|
      # load creds file, which you should really have
      creds_file_path = [nil, ''].include?(ENV['OPENSTACK_CREDS']) ? OPENSTACK_CRED_FILE : ENV['OPENSTACK_CREDS']

      # read in all the lines that look like FOO=BAR as a hash
      creds = File.exist?(creds_file_path = File.expand_path(creds_file_path)) ?
        Hash[*(File.open(creds_file_path).readlines.map{ |l| l.strip!; l.split('=') }.flatten)] : {}
      voc = vagrant_openshift_config['openstack']

      override.vm.box = voc["box_name"] || "openstack-dummy-box"
      override.vm.box_url = voc["box_url"] || OPENSTACK_BOX_URL
      # Make sure the private key from the key pair is provided
      override.ssh.private_key_path = creds['OSPrivateKeyPath'] || "~/.ssh/id_rsa"

      os.endpoint     = ENV['OS_AUTH_URL'] ? "#{ENV['OS_AUTH_URL']}/tokens" : creds['OSEndpoint']
      os.tenant       = ENV['OS_TENANT_NAME'] || creds['OSTenant']
      os.username     = ENV['OS_USERNAME']    || creds['OSUsername']
      os.api_key      = ENV['OS_PASSWORD']    || creds['OSAPIKey']
      os.keypair_name = voc['key_pair']       || creds['OSKeyPairName'] || "<OSKeypair>" # as stored in Nova
      os.flavor       = vagrant_openshift_config['instance_type']  || creds['OSFlavor']   || /m1.small/       # Regex or String
      os.image        = voc['image']          || creds['OSImage']    || /Fedora/         # Regex or String
      os.ssh_username = user = voc['ssh_user']|| creds['OSSshUser']  || "root"           # login for the VM instance
      os.server_name  = ENV['OS_HOSTNAME']    || vagrant_openshift_config['instance_name'] # name for the instance created
      full_provision(override.vm, user)

      # floating ip usually needed for accessing machines
      floating_ip     = creds['OSFloatingIP'] || ENV['OS_FLOATING_IP']
      os.floating_ip  = floating_ip == ":auto" ? :auto : floating_ip
      floating_ip_pool = creds['OSFloatingIPPool'] || ENV['OS_FLOATING_IP_POOL']
      os.floating_ip_pool = floating_ip_pool == "false" ? false : floating_ip_pool
    end if vagrant_openshift_config['openstack']


    # #########################
    # Set AWS provider settings
    config.vm.provider "aws" do |aws, override|
      creds_file_path = ENV['AWS_CREDS'].nil? || ENV['AWS_CREDS'] == '' ? AWS_CRED_FILE : ENV['AWS_CREDS']
      if File.exist?(File.expand_path(creds_file_path))
        aws_creds_file = Pathname.new(File.expand_path(creds_file_path))
        aws_creds      = aws_creds_file.exist? ? Hash[*(File.open(aws_creds_file.to_s).readlines.map{ |l| l.strip!
                                                          l.split('=') }.flatten)] : {}

        voc = vagrant_openshift_config['aws']
        override.vm.box               = voc['box_name'] || "aws-dummy-box"
        override.vm.box_url           = voc['box_url'] || AWS_BOX_URL
        override.vm.synced_folder sync_from, sync_to, disabled: true # rsyncing to public cloud not a great experience, use git
        override.ssh.username         = vagrant_openshift_config['aws']['ssh_user']
        override.ssh.private_key_path = aws_creds["AWSPrivateKeyPath"] || "PATH TO AWS KEYPAIR PRIVATE KEY"
        override.ssh.insert_key = true

        aws.access_key_id     = aws_creds["AWSAccessKeyId"] || "AWS ACCESS KEY"
        aws.secret_access_key = aws_creds["AWSSecretKey"]   || "AWS SECRET KEY"
        aws.keypair_name      = aws_creds["AWSKeyPairName"] || "AWS KEYPAIR NAME"
        aws.ami               = voc['ami']
        aws.region            = voc['ami_region']
        aws.subnet_id         = ENV['AWS_SUBNET_ID'] || vagrant_openshift_config['aws']['subnet_id'] || "subnet-cf57c596"
        aws.instance_type     = ENV['AWS_INSTANCE_TYPE'] || vagrant_openshift_config['instance_type'] || "t2.medium"
        aws.instance_ready_timeout = 240
        aws.tags              = { "Name" => ENV['AWS_HOSTNAME'] || vagrant_openshift_config['instance_name'] }
        aws.user_data         = %{
#cloud-config

growpart:
  mode: auto
  devices: ['/']
runcmd:
- [ sh, -xc, "sed -i s/^Defaults.*requiretty/\#Defaults\ requiretty/g /etc/sudoers"]
        }
        aws.block_device_mapping = [
          {
             "DeviceName" => "/dev/sda1",
             "Ebs.VolumeSize" => vagrant_openshift_config['volume_size'] || 25,
             "Ebs.VolumeType" => "gp2"
          }
        ]
      end
      #full_provision(override.vm)
    end if vagrant_openshift_config['aws']

end
