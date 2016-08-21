#! /bin/bash

# Copyright 2016 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

kubectl exec mysql-0 -- mysql -u root -e "create database test;"
kubectl exec mysql-1 -- mysql -u root -e "use test; create table pet (id int(10), name varchar(20));"
kubectl exec mysql-1 -- mysql -u root -e "use test; insert into pet (id, name) values (1, \"galera\");"
kubectl exec mysql-2 -- mysql -u root -e "use test; select * from pet;"

