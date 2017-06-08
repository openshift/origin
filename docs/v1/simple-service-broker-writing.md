# Use Cases For Simple Service Broker Writing
Jay Judkowitz

Nov 7th, 2016

## Introduction
At the Catalog SIG offsite, I brought up the topic that we need to have some way to make service broker authoring simple
(whether part of the Catalog deliverables or separate) or that we may not get people to adopt the technology.  There was some 
pushback that this was already handled and not a challenge and that I should put forth use cases that we can test the current 
plans against.  This document is the response to that.

## Multi-tenancy
It is clear that we can have some default service broker that always returns the same credentials for all binding apps.  This 
makes perfect sense for single tenant applications.  The credentials can be determined by the service instantiator at 
provisioning time and those can be stashed away and returned on any bind call.  But, what do we do for multi-tenant services?  

What is the easiest way to write a broker that can create unique credentials for each bind call
* Possibly creating everything itself
* Possibly some mix of creating things itself and taking inputs from the binding application.  For example, imagine the bind 
call schema asking the binding application to pass in a new username, but then the service broker generates the password and 
both the username and password are stored as secrets.

## Implementing binding mechanics
How does the service provider implement binding specific logic that goes beyond simply passing back credentials?  Perhaps the 
service needs to provision some capacity, create database entries, create new users, etc….   We need a way to store that per 
binding application logic.

## Updates
The “U” in CRUD stands for Update.  The logic for create and delete is pretty simple and could be done in default brokers, but 
update would be special.  What rollout policy would be used?  Does there need to be some management of persistent volumes? 
Etc… The update logic needs to be easy to enter into the broker.
