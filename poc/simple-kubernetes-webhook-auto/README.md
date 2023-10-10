# Based on
https://github.com/slackhq/simple-kubernetes-webhook

# Current goal
The idea of this POC is to show how we can create Spire entry automatically and then use Spire certificate for mutating/admision webhook.

Expected order of actions:
1. Create cluster
2. Create Spire server
3. Create Spire entry template
4. Apply webhook configs
5. Deploy webhook server
6. Ensure Spire creates entry automatically using the `"spiffe.io/spiffe-id": "true"` label of webhook server
7. Ensure webhooks are working as expected (we can deploy multiple pods and look at the server logs to check if the mutations and validation are being applied) (TODO)

### Requirements
* Docker
* kubectl
* [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
* Go >=1.20 (optional)

## Usage
### Create Cluster
First, we need to create a Kubernetes cluster:
```
❯ make cluster
```

### Set up Spire
```
❯ kubectl apply -k ./spire
```

```
❯ kubectl apply -f ./spire/template/clusterspiffeid-template.yaml
```

### Apply Webhook configs
```
❯ kubectl apply -f hooks/mutating.config.yaml
❯ kubectl apply -f hooks/validating.config.yaml
```

### Deploy Webhook Server
```
❯ make deploy
```

Then, make sure the admission webhook pod is running (in the `default` namespace):
```
❯ kubectl get pods
NAME                                        READY   STATUS    RESTARTS   AGE
simple-kubernetes-webhook-77444566b7-wzwmx   1/1     Running   0          2m21s
```

You can stream logs from it:
```
❯ make logs

🔍 Streaming simple-kubernetes-webhook logs...
kubectl logs -l app=simple-kubernetes-webhook -f
...
2023/10/10 13:09:30 waitForCertificates: wait
2023/10/10 13:09:31 waitForCertificates: wait
2023/10/10 13:09:31 waitForCertificates: wait
2023/10/10 13:09:32 waitForCertificates: wait
2023/10/10 13:09:32 OnX509ContextWatchError: Watch error called
2023/10/10 13:09:32 OnX509ContextWatcherError error: rpc error: code = PermissionDenied desc = no identity issued
2023/10/10 13:09:32 waitForCertificates: wait
2023/10/10 13:09:33 waitForCertificates: wait
2023/10/10 13:09:33 waitForCertificates: wait
```

Spire entries that allows watcher to catch the certificate (see simple-kubernetes-webhook example with manually added entries):
```
Entry ID         : 1e741294-f751-4d3b-84d3-ae53f13ffc76
SPIFFE ID        : spiffe://example.org/ns/spire/sa/spire-agent
Parent ID        : spiffe://example.org/spire/server
Revision         : 0
X509-SVID TTL    : default
JWT-SVID TTL     : default
Selector         : k8s_sat:agent_ns:spire
Selector         : k8s_sat:agent_sa:spire-agent
Selector         : k8s_sat:cluster:demo-cluster

Entry ID         : 05759006-7e56-4d0d-93e3-936873812c82
SPIFFE ID        : spiffe://example.org/ns/default/sa/default
Parent ID        : spiffe://example.org/ns/spire/sa/spire-agent
Revision         : 0
X509-SVID TTL    : default
JWT-SVID TTL     : default
Selector         : k8s:ns:default
Selector         : k8s:sa:default
DNS name         : simple-kubernetes-webhook.default.svc
```

Spire entry created using current template:
```
Entry ID         : 375d3899-57e2-40d4-a3d3-5e316f611cfb
SPIFFE ID        : spiffe://example.org/ns/default/pod/simple-kubernetes-webhook-fdc94c788-b96nh
Parent ID        : spiffe://example.org/spire/agent/k8s_psat/example/dba18745-2467-481b-9bbb-d6595765622f
Revision         : 0
X509-SVID TTL    : default
JWT-SVID TTL     : default
Selector         : k8s:pod-uid:2958af76-f969-4add-aede-3f0c60619ef2
DNS name         : simple-kubernetes-webhook.default.svc
```

TODO: The idea is to change the template (./spire/template/clusterspiffeid-template.yaml) to give `OnX509ContextWatcher` enough rights to avoid a PermissionDenied error.

#### Verify automatically created entries
```
❯ kubectl exec -n spire spire-server-0 --  /opt/spire/bin/spire-server entry show
```
