package com.example;

import java.util.ArrayList;
import java.util.List;

public class Memcached52371Status {

    // Add Status information here
    // Nodes are the names of the memcached pods
    private List<String> nodes;

    public List<String> getNodes() {
        if (nodes == null) {
            nodes = new ArrayList<>();
        }
        return nodes;
    }

    public void setNodes(List<String> nodes) {
        this.nodes = nodes;
    }
}
