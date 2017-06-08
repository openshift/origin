# A Year In The Life Of A Kubernetes Service Developer

I'm a developer in an enterprise environment.  For some fun in my spare time, I’m building a node.js chat application for me 
and a couple of coworkers to goof around with.  It’s really simple.  It runs on one container.  To get going, I simply 
download a node.js container onto minikube on my laptop and started coding.  I want some co-workers to be able to find the 
chat app to connect.  So, I register it as a service in kubernetes which is nice since it gives a persistent name for the
endpoint if the IP and port change.

The folks I had invited to use the service and were having fun with it and told people from another department about it and
now some of them want to join.  With all these people using the app, it’s more of a problem when a message is dropped.
Every time the system glitches or a client loses connection, the message is lost to the ether.  I really should connect this
to a message queue so that I can make sure that messages are not deleted on the server side until I’m assured they’ve been 
delivered.  I hoping to avoid the work of picking a system and learning it.  The good news is that when I was looking at the 
kubernetes service catalog the other day, I saw a RabbitMQ instance that someone else was maintaining.  All I needed to do 
was issue a single command to bind my chat app to the Rabbit service (and, of course, edit my software to use Rabbit).  The 
bind call gave back credentials which were automatically discovered by my chat app and it was off to the races.  I have no 
idea where that thing is running or how the firewalls work, but whatever, I trust it since it’s maintained by corporate IT.

To my surprise, this app started to really take off.  People were having so much fun with it that it became part of company 
morale.  I was assigned maintaining the app as a 20% project and even have a couple interns working with me on bug fixing.  
I now get paid to play with my service which is cool, but now I’m starting to get support tickets for it.

Now I need to maintain multiple instances of this.  I have test and staging instances.  I also have two to three versions 
under development at any given time.  I can deploy this thing with my eyes closed, but the interns have been accidentally 
creating some difficulties for me.  They sometimes do strange configuration of test instances.  When I ask them to be be 
more mindful about deployment, they push back on me.  They tell me that the company has adopted a standardized way to 
expose services to others that (a) handles parameterization and (b) creates a standard provisioning API so that they can
deploy my chat service the same way that they deploy everything else in the company.  I check it out and it’s pretty cool -
creating the broker and setting up the parameters to deploy the service on command is a <simple process>.  Now I can deploy
the service with a single command.  Even though it was not hard to deploy for me before, it’s easier now even and the
interns are always deploying within the constraints I set.  And, most importantly, the interns nowhave no problem deploying 
the service as intended and are not making deployment mistakes.

Someone in tech support just had the idea that they can use my chat service inside the call center system.  They need to
connect their app to my chat service the same way that I connected to the Rabbit service that IT maintains for me.  I call
up the Rabbit person and ask how they made that binding thing work.  Luckily, it was not hard at all.  I just did <simple
workflow> to update the service broker to respond to binding requests, deployed an instance and the customer support people
were off to the races.

To be continued...
