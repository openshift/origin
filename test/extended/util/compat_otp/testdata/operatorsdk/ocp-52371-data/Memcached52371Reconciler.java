package com.example;

import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;

import io.fabric8.kubernetes.api.model.ContainerBuilder;
import io.fabric8.kubernetes.api.model.ContainerPortBuilder;
import io.fabric8.kubernetes.api.model.LabelSelectorBuilder;
import io.fabric8.kubernetes.api.model.ObjectMetaBuilder;
import io.fabric8.kubernetes.api.model.OwnerReferenceBuilder;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodSpecBuilder;
import io.fabric8.kubernetes.api.model.PodTemplateSpecBuilder;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.api.model.apps.DeploymentBuilder;
import io.fabric8.kubernetes.api.model.apps.DeploymentSpecBuilder;
import org.apache.commons.collections.CollectionUtils;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

public class Memcached52371Reconciler implements Reconciler<Memcached52371> { 
  private final KubernetesClient client;

  public Memcached52371Reconciler(KubernetesClient client) {
    this.client = client;
  }

  // TODO Fill in the rest of the reconciler

  @Override
  public UpdateControl<Memcached52371> reconcile(
      Memcached52371 resource, Context context) {
      // TODO: fill in logic
      Deployment deployment = client.apps()
              .deployments()
              .inNamespace(resource.getMetadata().getNamespace())
              .withName(resource.getMetadata().getName())
              .get();

      if (deployment == null) {
          Deployment newDeployment = createMemcached52371Deployment(resource);
          client.apps().deployments().create(newDeployment);
          return UpdateControl.noUpdate();
      }

      int currentReplicas = deployment.getSpec().getReplicas();
      int requiredReplicas = resource.getSpec().getSize();

      if (currentReplicas != requiredReplicas) {
          deployment.getSpec().setReplicas(requiredReplicas);
          client.apps().deployments().createOrReplace(deployment);
          return UpdateControl.noUpdate();
      }

      List<Pod> pods = client.pods()
          .inNamespace(resource.getMetadata().getNamespace())
          .withLabels(labelsForMemcached52371(resource))
          .list()
          .getItems();

      List<String> podNames =
          pods.stream().map(p -> p.getMetadata().getName()).collect(Collectors.toList());


      if (resource.getStatus() == null
               || !CollectionUtils.isEqualCollection(podNames, resource.getStatus().getNodes())) {
           if (resource.getStatus() == null) resource.setStatus(new Memcached52371Status());
           resource.getStatus().setNodes(podNames);
           return UpdateControl.updateResource(resource);
      }

      return UpdateControl.noUpdate();
  }

  private Map<String, String> labelsForMemcached52371(Memcached52371 m) {
    Map<String, String> labels = new HashMap<>();
    labels.put("app", "memcached");
    labels.put("memcached_cr", m.getMetadata().getName());
    return labels;
  }

  private Deployment createMemcached52371Deployment(Memcached52371 m) {
    return new DeploymentBuilder()
        .withMetadata(
            new ObjectMetaBuilder()
                .withName(m.getMetadata().getName())
                .withNamespace(m.getMetadata().getNamespace())
                .withOwnerReferences(
                    new OwnerReferenceBuilder()
                        .withApiVersion("v1")
                        .withKind("Memcached52371")
                        .withName(m.getMetadata().getName())
                        .withUid(m.getMetadata().getUid())
                        .build())
                .build())
        .withSpec(
            new DeploymentSpecBuilder()
                .withReplicas(m.getSpec().getSize())
                .withSelector(
                    new LabelSelectorBuilder().withMatchLabels(labelsForMemcached52371(m)).build())
                .withTemplate(
                    new PodTemplateSpecBuilder()
                        .withMetadata(
                            new ObjectMetaBuilder().withLabels(labelsForMemcached52371(m)).build())
                        .withSpec(
                            new PodSpecBuilder()
                                .withContainers(
                                    new ContainerBuilder()
                                        .withImage("quay.io/olmqe/memcached-docker:multi-arch")
                                        .withName("memcached")
                                        .withCommand("memcached", "-o", "modern", "-v")
                                        .withPorts(
                                            new ContainerPortBuilder()
                                                .withContainerPort(11211)
                                                .withName("memcached")
                                                .build())
                                        .build())
                                .build())
                        .build())
                .build())
        .build();
  }

}
