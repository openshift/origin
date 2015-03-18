HA Router and Failover
======================
This readme describes steps to add multiple HA OpenShift routers with
failover capability to achieve several nines of availability. 


Pre-requisites/Prep Time
------------------------

1. Launch a OpenShift cluster via whatever mechanism you use. The steps
   below assume you are doing this on a dev machine using vagrant.

        export OPENSHIFT_DEV_CLUSTER=1
        cd $this-repo-git-co-dir  #  cloned from git@github.com:ramr/origin
        vagrant up

2. Wait for the cluster to come up (I'm just sitting here watching the
   wheels go round and round). And then people say am crazy ... so start
   the sample app. For demo purposes (as of now), this simulates the
   openshift router. This step is for *DEMO* purposes only.

        ```  
        vagrant ssh minion-1  
       
        #  And run this setup on each of the minions.  
        #  *This step is harmful - use with caution.*  
        #  "OkOk"  
        sudo docker rm -f $(sudo docker ps -q -s)  
  
        #  "What?!?!" This one's ok - you can use with disdain.  
        sudo docker run -dit -p 80:8080 openshift/hello-openshift  
  
        #  Check that the app is working ...  
        curl http://localhost/
  
        #  Rinse-lather-repeat above step(s) on the other minion[s].  
        #   vagrant ssh minion-2  
        ```

3. Ensure you can get to the hello openshift app from inside/outside the vm.

        echo "minion-{1,2} use IPs 10.245.2.{3,4} in the dev environment."
        curl -k http://10.245.2.3/
        curl -k http://10.245.2.4/


HA Routing Failover Setup (*WIP*)
=================================

1. Copy the router HA settings example config and edit it as needed.

        cd /vagrant/hack/exp/router-failover
        cp conf/settings.example  settings.minion-1
        cp conf/settings.example  settings.minion-2
        #
        #  And as per your environment, set/edit the values for
        #    ADMIN_EMAILS, EMAIL_FROM, SMTP_SERVER,
        #    PRIMARY_HA_VIPS, SLAVE_HA_VIPS and INTERFACE.

2. For demo purposes, we are going to flip the PRIMARY and SLAVE groups
   on minion-2 ... this allows both minions to serve in an Active-Active
   fashion.

        ```
        #  Flip PRIMARY+SLAVE groups on minion-2 ("Papoy?! Ah Papoy!!").  
        sed -i "s/^PRIMARY_GROUPS=\(.*\)/PRIMARY_GROUPS_OLD=\1/g;  
                s/^SLAVE_GROUPS=\(.*\)/PRIMARY_GROUPS=\1/g;  
                s/^PRIMARY_GROUPS_OLD=\(.*\)/SLAVE_GROUPS=\1/g;" \  
            settings.minion-2  
  
        #  diff conf/settings.example  settings.minion-{1,2}  
        ```

3. Setup router HA with failover using the 2 config files we created.

        ```  
        #  Run these commands on the minions via vagrant ssh minion-{1,2}  
        #    cd /vagrant/hack/exp  
        #    sudo ./failover-setup.sh settings.minion-{1,2}  
          
        #  OkOk - minion-1  
        vagrant ssh minion-1 -c "cd /vagrant/hack/exp/router-failover;  \  
            sudo ./failover-setup.sh settings.minion-1"  
  
        #  And on minion-2  
        vagrant ssh minion-2 -c "cd /vagrant/hack/exp/router-failover;  \  
            sudo ./failover-setup.sh settings.minion-2"  
        ```

4. Check that you can get to the hello openshift app using the VIPs from
   inside/outside the vms.

        curl -k http://10.245.2.111/
        curl -k http://10.245.2.222/
        curl -k http://10.245.2.223/
        curl -k http://10.245.2.90/
        #  YAY

5. On each minion, you can check what VIPs are being serviced by that
   minion via `ip a s enp0s8`. Substitute the appropriate interface name
   for `enp0s8` in your environment.

        echo "Aww - minion-1"
        vagrant ssh minion-1 -c "ip a s enp0s8"
  
        echo "And on minion-2"
        vagrant ssh minion-2 -c "ip a s enp0s8"


HA Routing Failover Demo
========================

1. Stop the router on one of the minions. In the case of our demo, this is
   aka hello-openshift app ("minions laughing").

        ```  
        #  sudo docker rm -f $(sudo docker ps -a | grep openshift/hello-openshift | awk '{print $1}')  
  
        vagrant ssh minion-1  
        cid=$(sudo docker ps -a | grep openshift/hello-openshift | awk '{print $1}')  
        [ -n "$cid" ] && sudo docker rm -f $cid  
        ```
 

2. Check that you can get to the hello openshift app using the VIPs from
   inside/outside the vms.

        curl -k http://10.245.2.111/
        curl -k http://10.245.2.222/
        curl -k http://10.245.2.223/
        curl -k http://10.245.2.90/
        #  "YAY"

3. Bring back the router ("minions laughing" - ok the hello-openshift app).

        sudo docker run -dit -p 80:8080 openshift/hello-openshift

4. Check that you can get to the hello openshift app using the VIPs from
   inside/outside the vms.

        curl -k http://10.245.2.111/
        curl -k http://10.245.2.222/
        curl -k http://10.245.2.223/
        curl -k http://10.245.2.90/
        #  "YAY"

5. Halt one of the minions ("Aww") ...

        vagrant ssh halt minion-2

6. Check that you can _still_ get to the hello openshift app using the
   VIPs from inside/outside the vms.

        curl -k http://10.245.2.111/
        curl -k http://10.245.2.222/
        curl -k http://10.245.2.223/
        curl -k http://10.245.2.90/
        #  "YAY"


TODOs/Edge CASES:
-----------------

*Beware of the dog - it bites! You have been warned*  
There's a 2 second delay (process existence check) as of now,  we can tune
this up/down appropriately.  
And it is pertinent to mention here that this solution is not true
fault-tolerance (100% availbility) - its just failover capability to
provide high availability (99.[9]n% availability - cheaper but not perfect).
So be aware of this and use it appropriately within your environment.

One alternative to achieve several more 9s of availability is to
  * stop keepalived immediately if the router or the docker container
    running the router goes down.
  * And start keepalived start it when the router comes back up because
    the replication controller notices things ain't kosher.


*Sound Effects*
Link quoted sound effects (ala "OkOk") to
    http://www.soundboard.com/sb/minions
