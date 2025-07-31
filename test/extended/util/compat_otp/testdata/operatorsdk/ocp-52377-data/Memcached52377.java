package com.example;

import io.fabric8.kubernetes.api.model.Namespaced;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.model.annotation.Group;
import io.fabric8.kubernetes.model.annotation.Version;

@Version("v1")
@Group("cache.example.com")
public class Memcached52377 extends CustomResource<Memcached52377Spec, Memcached52377Status> implements Namespaced {}

