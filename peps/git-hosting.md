Hosting on Openshift V3
=======================

__Requirements__  

- Scalablilty  
  - support number of users in orders of Openshift online.

- Failover  
  - At minimum back-ups of user, permissions, hooks and repo data.

- User experience  
  - SSH
  - HTTP/HTTPS
  - Access control - At minimum we need to be able keep each user's repo secure.
  - Push/Web hooks
  - Web UI - needed yes/no?

- Admin experience
  - Ease of setup
  - Administration capability

  
Gitblit
-------
Gitblit is an open-source, pure Java stack for managing, viewing, and serving Git repositories.
It's designed primarily as a tool for small workgroups who want to host centralized repositories.

__Out of the box__

- Supports HTTP(S) and SSH
- Supports users, teams and per repository permissions (data stored locally to users.conf file)
- Six per user/team repository access permissions

    V (view in web ui, RSS feeds, download zip)
    R (clone)
    RW (clone and push)
    RWC (clone and push with ref creation)
    RWD (clone and push with ref creation, deletion)
    RW+ (clone and push with ref creation, deletion, rewind

- Groovy pre and post push hook scripts, per-repository or globally for all repositories (Still need to figure out where these are stored)
- Web UI
- Not scalable.  It is meant for small workgroups.
- Back-up strategy - clone repositories and keep them in sync from one Gitblit instance to another using their federation mechanism. Gitblit federation is like a master-slave system.  Where updates are expected to always go to the master, except that in Gitblit federation instead of master pushing the updates to the slave, the slave pulls the update rom master at a configurable interval (minimum 5 minutes).
- Easy to setup
- It has admin account and UI console (It does not look like it would scale)

__Make it work__

- Write our own authentication plugin to authenticate with external authentication server
- Write our own plugin to store user, team and permissions to a central location (most likely a DB)
- The Gitblit federation mechanism does not meet our needs. Rather than the back-up server pulling updates at regular intervals, it is preferable to send updates to the back-up server with each push.  One possibility is to write a custom hook that run for all repos and push updates to back-up server.  Still looking at other alternatives...
- Scaling strategy
  - Option 1:
    - Distribute user repositories across serveral distinct instances of Gitblit
    - Need to define and implement algorithm for density management
  - Option 2:
    - Use DFS for repository storage. 
    - Loadbalance SSH and HTTP(s) connections.


GitLab (Community addition)
---------------------------


SCM Manager
-----------



    
  

