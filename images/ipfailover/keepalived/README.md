HA Router and Failover
======================
This readme describes steps to add multiple HA OpenShift routers with
failover capability to achieve several nines of availability.


Build and Test
--------------
1.  Verify docker image build and run tests.

        $ make -f makefile.test  #  or make -f makefile.test image
        $ make -f makefile.test test


Pre-requisites/Prep Time
------------------------

1. Launch a OpenShift cluster via whatever mechanism you use. The steps
   below assume you are doing this on a dev machine using vagrant.

        $ export OPENSHIFT_DEV_CLUSTER=1
        $ cd $this-repo-git-co-dir  # cloned from git@github.com:ramr/origin
        $ vagrant up


2. Wait for the cluster to come up and then start the OpenShift router
   with two (_2_) replicas.

        $ vagrant ssh minion-1  # (or master or minion-2).
        #  Ensure KUBECONFIG is set or else set it.
        [ -n "$KUBECONFIG" ] ||  \
           export KUBECONFIG=/origin.local.config/master/admin.kubeconfig
        #  openshift kube get dc,rc,pods,se,mi,routes
        oadm router arparp --create --replicas=2  \
                                   --credentials="${KUBECONFIG}"


3. Wait for the Router pods to get into running state (I'm just sitting
   here watching the wheels go round and round).

        $ vagrant ssh minion-1 # (or master or minion-2).
        pods="openshift/origin-haproxy-router|openshift/origin-deployer"
        while openshift kube get pods | egrep -e "$pods" |   \
                grep "Pending" > /dev/null; do
            echo -n "."
            #  "OkOk"
            sleep 1
        done
        echo ""


4. Check that the two OpenShift router replicas are up and serving.

        $ #  This will be a bit slow, but it should return a 503 HTTP code
        $ #  indicating that haproxy is serving on port 80.
        $ vagrant ssh minion-1
        sudo docker ps  | grep "openshift/origin-haproxy-router"
        curl -s -o /dev/null -w "%{http_code}\n"  http://localhost/

        $ #  Repeat on minion-2:
        $ vagrant ssh minion-2
        sudo docker ps  | grep "openshift/origin-haproxy-router"
        curl -s -o /dev/null -w "%{http_code}\n"  http://localhost/


5. Create an user, project and app.

        $ vagrant ssh minion-1
        #  Add user and project.
        oadm policy add-role-to-user view test-admin
        oadm new-project test --display-name="Failover Sample" \
           --description="Router Failover" --admin=test-admin
        #  Create a test app using the template.
        cd /vagrant/hack/exp/router-failover
        oc create -n test -f conf/hello-openshift-template.json

        echo "Wait for the app to startup and check app is reachable."
        for ip in 10.245.2.3 10.245.2.4; do
          curl -H "Host: hello.openshift.test" -o /dev/null -s -m 5  \
               -w "%{http_code}\n" http://$ip/
        done
        echo "Ensure HTTP status code is 200 for both http://10.245.2.{3,4}"
        #  && echo "YAY"


6. Ensure you can get to the hello openshift app from inside/outside the vm.

        $ #  minion-{1,2} use IPs 10.245.2.{3,4} in the dev environment.
        for ip in 10.245.2.3 10.245.2.4; do
          echo "$ip: $(curl -s --resolve hello.openshift.test:80:$ip  \
                            -m 5 http://hello.openshift.test)"
        done


HA Routing Failover Setup
=========================

1. Copy the router HA settings example config and edit it as needed.

        $ cd /vagrant/hack/exp/router-failover
        $ cp conf/settings.example  settings.minion-1
        $ cp conf/settings.example  settings.minion-2
        $ #
        $ #  And as per your environment, set/edit the values for
        $ #    ADMIN_EMAILS, EMAIL_FROM, SMTP_SERVER,
        $ #    PRIMARY_HA_VIPS, SLAVE_HA_VIPS and INTERFACE.

2. For demo purposes, we are going to flip the PRIMARY and SLAVE groups
   on minion-2 ... this allows both minions to serve in an Active-Active
   fashion.

        $ #  Flip PRIMARY+SLAVE groups on minion-2 ("Papoy?! Ah Papoy!!").
        $ sed -i "s/^PRIMARY_GROUPS=\(.*\)/PRIMARY_GROUPS_OLD=\1/g;
                  s/^SLAVE_GROUPS=\(.*\)/PRIMARY_GROUPS=\1/g;
                  s/^PRIMARY_GROUPS_OLD=\(.*\)/SLAVE_GROUPS=\1/g;" \
              settings.minion-2

        $ #  Check what the differences are on the minions.
        $ diff conf/settings.example  settings.minion-1
        $ diff conf/settings.example  settings.minion-2


3. Optionally clear the config - just so that we have a completely clean
   slate. Step 4 below does this - but this is here just for my demo env
   reuse purposes.

        $ #  Run these commands on the minions via vagrant ssh minion-{1,2}
        $ #    sudo service keepalived stop
        $ #    sudo rm -f /etc/keepalived/keepalived.conf

        $ #  OkOk
        for m in minion-1 minion-2; do
           vagrant ssh $m -c "sudo service keepalived stop;  \
                              sudo rm -f /etc/keepalived/keepalived.conf"
        done


4. Setup router HA with failover using the 2 config files we created.

        $ #  Run these commands on the minions via vagrant ssh minion-{1,2}
        $ #    cd /vagrant/hack/exp/router-failover
        $ #    sudo ./failover-setup.sh settings.minion-{1,2}

        $ #  OkOk - minion-1
        for m in minion-1 minion-2; do
           vagrant ssh $m -c "cd /vagrant/hack/exp/router-failover;  \
                              sudo ./failover-setup.sh settings.$m"
        done


5. On each minion, you can check what VIPs are being serviced by that
   minion via `ip a ls dev enp0s8`. Substitute the appropriate interface
   name for `enp0s8` in your environment.

        $ #  "minions laughing" ...
        for m in minion-1 minion-2; do
           vagrant ssh $m -c "ip a ls dev enp0s8"
        done


6. Check that you can get to the hello openshift app using the VIPs from
   inside/outside the vms.

        for ip in 10.245.2.90 10.245.2.111 10.245.2.222 10.245.2.223; do
          echo "$ip: $(curl -s --resolve hello.openshift.test:80:$ip  \
                            -m 5 http://hello.openshift.test)"
        done
        #  && echo "YAY"


HA Routing Failover Demo
========================
Whilst following the steps below, you can also monitor one of the VIPs on a
terminal on your host system. This just busy loops sending requests to a
specific VIP.

        tko="--connect-timeout 2"  #  Maybe use -m 2 instead.
        resolver="--resolve hello.openshift.test:80:10.245.2.111"
        while true; do
          echo "$(date): $(curl -s $tko $resolver hello.openshift.test)"
        done | tee /tmp/foo


HA Simple Failover Test (keepalived)
====================================
The simplest test on VIP failover is to stop keepalived on one of the
minions.

        $ vagrant ssh minion-1

        $ #  Check which VIPs are served by this minion.
        ip a ls dev enp0s8

        $ #  Make sure the VIP in the busy loop above 10.245.2.111 is
        $ #  "owned"/serviced by this minion. Or then use a VIP that's
        $ #  serviced by this minion in the above mentioned busy looper
        $ #  monitoring script (while true; curl ... done).
        sudo service keepalived stop

        $ vagrant ssh minion-2
        #  Check that the VIPs from minion-1 are taken over by this minion.
        ip a ls dev enp0s8

        $ vagrant ssh minion-1
        $ #  Set things back to a "good" state by starting back keepalived.
        sudo service keepalived start

        $ #  Check the VIPs served by this minion.
        ip a ls dev enp0s8


HA Hard Failover Test (bring down the minion)
=============================================
The hard failover VIP test basically involves stopping the whole shebang
(keepalived, openshift-router and haproxy) by bringing down one of
the minions.

1. Halt one of the minions ("Aww") ...

        $ #  If you are monitoring a specific VIP ala 10.245.2.111 in the
        $ #  example mentioned above, then bring down the minion that's
        $ #  "owns" that VIP. For now, bringing a random one down.
        $ vagrant halt minion-$((RANDOM%2 + 1))


2. Check that you can still get to the hello openshift app using the VIPs
   from inside/outside the vms.

        for ip in 10.245.2.90 10.245.2.111 10.245.2.222 10.245.2.223; do
          echo "$ip: $(curl -s --resolve hello.openshift.test:80:$ip  \
                            -m 5 http://hello.openshift.test)"
        done
        $ #  && echo "YAY"


3. Bring back the minion ("YAY") ...

        $ vagrant up minion-{1,2}


4. Wait for the minion to come back online.

5. Check how the VIPs are balanced between the 2 minions.

        for m in minion-1 minion-2; do
          vagrant ssh $m -c "ip a ls dev enp0s8"
        done

6. Check that you can still get to the hello openshift app using the VIPs
   from inside/outside the vms.

        for ip in 10.245.2.90 10.245.2.111 10.245.2.222 10.245.2.223; do
          echo "$ip: $(curl -s --resolve hello.openshift.test:80:$ip  \
                            -m 5 http://hello.openshift.test)"
        done
        $ #  && echo "YAY"



HA Soft Failover Test
=====================

1. Eventually this would test the keepalived process - but for now this
   just shows how long the Kubernetes Replication Controller takes to
   restart the services.

        $ #  Stop the router on one of the minions ("Aaw").
        $ vagrant ssh minion-$((RANDOM%2 + 1))
        sudo kill -9 $(ps -e -opid,args | grep openshift-router |  \
                          grep -v grep | awk '{print $1}')
        $ # OR:
        sudo docker rm -f $(sudo docker ps |  \
                               grep openshift/origin-haproxy-router |  \
                               awk '{print $1}')

2. Check that you can still get to the hello openshift app using the VIPs
   from inside/outside the vms.

        for ip in 10.245.2.90 10.245.2.111 10.245.2.222 10.245.2.223; do
          echo "$ip: $(curl -s --resolve hello.openshift.test:80:$ip  \
                            -m 5 http://hello.openshift.test)"
        done
        $ #  && echo "YAY"
        $ #  Wait for the router to come back up and run above check again.



TODOs/Edge CASES:
-----------------

##  *Beware of the dog - it bites! You have been warned*
There's a 2 second delay (process existence check) as of now, we can
tune this up/down appropriately.
And it is pertinent to mention here that this solution is not true
fault-tolerance (100% availbility) - its just failover capability to
provide high availability (99.[9]{n}% availability - cheap but by no
means perfect).
So be aware of this and use it appropriately within your environment.

One alternative to achieve several more 9s of availability is to
  * stop keepalived immediately if the router or the docker container
    running the router goes down.
  * And start keepalived start it when the router comes back up because
    the replication controller notices things ain't kosher.
But the bang for buck here is a low.


##  *Sound Effects*
Link quoted sound effects (ala "OkOk") to

        http://www.soundboard.com/sb/minions

