#Definitions

##Report

| Name              | Description                                                                     | Required | Schema       | Default |
|-------------------|---------------------------------------------------------------------------------|----------|--------------|---------|
| Timestamp         | RFC 3339 date and time at which the object was created; populated by the system | false    | string       |         |
| PodRequirements   | Pod object attributes that can affect its scheduling                            | false    | PodResources |         |
| TotalInstances    | Number of schedulable instances                                                 | false    | integer      |         |
| NodesNumInstances | Map of node names and amount of pod instances which could be scheduled on them  | false    | JSON map     |         |
| FailReasons       | Failure reports                                                                 | false    | FailReasons  |         |

##PodResources

| Name   | Description                   | Required | Schema | Default |
|--------|-------------------------------|----------|--------|---------|
| Cpu    | Amount of cpu pod requires    | false    | string |         |
| Memory | Amount of memory pod requires | false    | string |         |

##FailReasons
| Name         | Description                                                                         | Required | Schema   | Default |
|--------------|-------------------------------------------------------------------------------------|----------|----------|---------|
| FailType     | Reason another instance of pod couldn't be scheduled                                | false    | string   |         |
| FailMessage  | Description message for FailType                                                    | false    | string   |         |
| NodeFailures | Map of node names and failures which led to not scheduling another instance on them | false    | JSON map |         |