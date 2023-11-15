# k8s-csi-demo

k8s CSI minimal demo

## prerequisite

This demo use the minimal deployment to implement k8s CSI, only valid at the `one node` scenario.  
For example you can use the [kind-config.yaml](/kind-config.yaml) to create a minimal k8s cluster.

The Architecture of CSI follows the third method from the [CSI spec](https://github.com/container-storage-interface/spec/blob/master/spec.md#architecture):  

```
                            CO "Node" Host(s)
+-------------------------------------------+
|                                           |
|  +------------+           +------------+  |
|  |     CO     |   gRPC    | Controller |  |
|  |            +----------->    Node    |  |
|  +------------+           |   Plugin   |  |
|                           +------------+  |
|                                           |
+-------------------------------------------+

Figure 3: Headless Plugin deployment, only the CO Node hosts run
Plugins. A unified Plugin component supplies both the Controller
Service and Node Service.
```

## references

- <https://github.com/container-storage-interface/spec/blob/master/spec.md>
- <https://kubernetes.io/docs/concepts/storage/volumes/#csi>
- <https://kubernetes.io/docs/concepts/storage/ephemeral-volumes/>
- <https://github.com/kubernetes-csi/csi-driver-host-path/tree/master>
