package com.example;

import io.fabric8.kubernetes.api.model.Namespaced;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.model.annotation.Group;
import io.fabric8.kubernetes.model.annotation.Version;

@Version("v1")
@Group("cache.example.com")
public class Memcached52371 extends CustomResource<Memcached52371Spec, Memcached52371Status> implements Namespaced {}

