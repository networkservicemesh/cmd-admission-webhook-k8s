# Based on
https://github.com/slackhq/simple-kubernetes-webhook

# Current goal
The idea of this POC is to show how we can use Spire certificate for mutating/admision webhook.

Order of actions:
1. Create cluster
2. Create Spire server
3. Create spire entry to allow webhook server watcher to catch Spire certificate, save it and use
4. Apply webhook configs
5. Deploy webhook server
6. Ensure webhooks are working as expected (we can deploy multiple pods and look at the server logs to check if the mutations and validation are being applied)


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

#### Create a new registration entry for the node, specifying the SPIFFE ID to allocate to the node:
```
❯ kubectl exec -n spire spire-server-0 -- /opt/spire/bin/spire-server entry create -spiffeID spiffe://example.org/ns/spire/sa/spire-agent -selector k8s_sat:cluster:demo-cluster -selector k8s_sat:agent_ns:spire -selector k8s_sat:agent_sa:spire-agent -node
```

#### Create a new registration entry for the workload, specifying the SPIFFE ID to allocate to the workload:
```
❯ kubectl exec -n spire spire-server-0 -- /opt/spire/bin/spire-server entry create -spiffeID spiffe://example.org/ns/default/sa/default -parentID spiffe://example.org/ns/spire/sa/spire-agent -selector k8s:ns:default -selector k8s:sa:default -dns simple-kubernetes-webhook.default.svc
```

#### Verify created entries
```
❯ kubectl exec -n spire spire-server-0 --  /opt/spire/bin/spire-server entry show
```
to remove if needed
```
❯ kubectl exec -n spire spire-server-0 --  /opt/spire/bin/spire-server entry delete -entryID ${ID}
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
2023/10/10 12:22:14 key: /etc/webhook/certs/tls.key
2023/10/10 12:22:14 file: /etc/webhook/certs/tls.crt
2023/10/10 12:22:14 waitForCertificates: wait
2023/10/10 12:22:14 waitForCertificates: wait
2023/10/10 12:22:15 waitForCertificates: wait
2023/10/10 12:22:15 OnX509ContextUpdate: Update called for dir /etc/webhook/certs
2023/10/10 12:22:15 writeToDisk: try to write
2023/10/10 12:22:15 Successfully wrote
time="2023-10-10T12:22:15Z" level=info msg="End watcher"
time="2023-10-10T12:22:15Z" level=info msg="Listening on port 443..."
time="2021-09-03T05:02:21Z" level=debug msg=healthy uri=/health
```

And hit it's health endpoint from your local machine:
```
❯ curl -k https://localhost:8443/health
OK
```

### Deploying pods
Deploy a valid test pod that gets successfully created:
```
❯ make pod
```
You should see in the admission webhook logs that the pod got mutated and validated.

example:
```
...
time="2023-10-10T12:23:48Z" level=debug msg="received mutation request" uri="/mutate-pods?timeout=2s"
time="2023-10-10T12:23:48Z" level=info msg="setting lifespan tolerations" min_lifespan=7 mutation=min_lifespan pod_name=lifespan-seven
time="2023-10-10T12:23:48Z" level=debug msg="pod env injected {KUBE true nil}" mutation=inject_env pod_name=lifespan-seven
time="2023-10-10T12:23:48Z" level=debug msg="sending response" uri="/mutate-pods?timeout=2s"
time="2023-10-10T12:23:48Z" level=debug msg="{\"kind\":\"AdmissionReview\",\"apiVersion\":\"admission.k8s.io/v1\",\"response\":{\"uid\":\"38c654e9-3c9e-414b-aea4-2a9f406ebe28\",\"allowed\":true,\"patch\":\"W3sib3AiOiJhZGQiLCJwYXRoIjoiL3NwZWMvY29udGFpbmVycy8wL2VudiIsInZhbHVlIjpbeyJuYW1lIjoiS1VCRSIsInZhbHVlIjoidHJ1ZSJ9XX0seyJvcCI6ImFkZCIsInBhdGgiOiIvc3BlYy90b2xlcmF0aW9ucy8tIiwidmFsdWUiOnsiZWZmZWN0IjoiTm9TY2hlZHVsZSIsImtleSI6ImFjbWUuY29tL2xpZmVzcGFuLXJlbWFpbmluZyIsIm9wZXJhdG9yIjoiRXF1YWwiLCJ2YWx1ZSI6IjE0In19LHsib3AiOiJhZGQiLCJwYXRoIjoiL3NwZWMvdG9sZXJhdGlvbnMvLSIsInZhbHVlIjp7ImVmZmVjdCI6Ik5vU2NoZWR1bGUiLCJrZXkiOiJhY21lLmNvbS9saWZlc3Bhbi1yZW1haW5pbmciLCJvcGVyYXRvciI6IkVxdWFsIiwidmFsdWUiOiIxMyJ9fSx7Im9wIjoiYWRkIiwicGF0aCI6Ii9zcGVjL3RvbGVyYXRpb25zLy0iLCJ2YWx1ZSI6eyJlZmZlY3QiOiJOb1NjaGVkdWxlIiwia2V5IjoiYWNtZS5jb20vbGlmZXNwYW4tcmVtYWluaW5nIiwib3BlcmF0b3IiOiJFcXVhbCIsInZhbHVlIjoiMTIifX0seyJvcCI6ImFkZCIsInBhdGgiOiIvc3BlYy90b2xlcmF0aW9ucy8tIiwidmFsdWUiOnsiZWZmZWN0IjoiTm9TY2hlZHVsZSIsImtleSI6ImFjbWUuY29tL2xpZmVzcGFuLXJlbWFpbmluZyIsIm9wZXJhdG9yIjoiRXF1YWwiLCJ2YWx1ZSI6IjExIn19LHsib3AiOiJhZGQiLCJwYXRoIjoiL3NwZWMvdG9sZXJhdGlvbnMvLSIsInZhbHVlIjp7ImVmZmVjdCI6Ik5vU2NoZWR1bGUiLCJrZXkiOiJhY21lLmNvbS9saWZlc3Bhbi1yZW1haW5pbmciLCJvcGVyYXRvciI6IkVxdWFsIiwidmFsdWUiOiIxMCJ9fSx7Im9wIjoiYWRkIiwicGF0aCI6Ii9zcGVjL3RvbGVyYXRpb25zLy0iLCJ2YWx1ZSI6eyJlZmZlY3QiOiJOb1NjaGVkdWxlIiwia2V5IjoiYWNtZS5jb20vbGlmZXNwYW4tcmVtYWluaW5nIiwib3BlcmF0b3IiOiJFcXVhbCIsInZhbHVlIjoiOSJ9fSx7Im9wIjoiYWRkIiwicGF0aCI6Ii9zcGVjL3RvbGVyYXRpb25zLy0iLCJ2YWx1ZSI6eyJlZmZlY3QiOiJOb1NjaGVkdWxlIiwia2V5IjoiYWNtZS5jb20vbGlmZXNwYW4tcmVtYWluaW5nIiwib3BlcmF0b3IiOiJFcXVhbCIsInZhbHVlIjoiOCJ9fSx7Im9wIjoiYWRkIiwicGF0aCI6Ii9zcGVjL3RvbGVyYXRpb25zLy0iLCJ2YWx1ZSI6eyJlZmZlY3QiOiJOb1NjaGVkdWxlIiwia2V5IjoiYWNtZS5jb20vbGlmZXNwYW4tcmVtYWluaW5nIiwib3BlcmF0b3IiOiJFcXVhbCIsInZhbHVlIjoiNyJ9fV0=\",\"patchType\":\"JSONPatch\"}}" uri="/mutate-pods?timeout=2s"
```

Deploy a non valid pod that gets rejected:
```
❯ make bad-pod
```
You should see in the admission webhook logs that the pod validation failed. It's possible you will also see that the pod was mutated, as webhook configurations are not ordered.

example:
```
time="2023-10-10T12:26:21Z" level=debug msg="received mutation request" uri="/mutate-pods?timeout=2s"
time="2023-10-10T12:26:21Z" level=info msg="no lifespan label found, applying default lifespan toleration" min_lifespan=0 mutation=min_lifespan pod_name=offensive-pod
time="2023-10-10T12:26:21Z" level=debug msg="pod env injected {KUBE true nil}" mutation=inject_env pod_name=offensive-pod
time="2023-10-10T12:26:21Z" level=debug msg="sending response" uri="/mutate-pods?timeout=2s"
time="2023-10-10T12:26:21Z" level=debug msg="{\"kind\":\"AdmissionReview\",\"apiVersion\":\"admission.k8s.io/v1\",\"response\":{\"uid\":\"eabdee00-2d4f-4ed7-95fa-8b73ae13503e\",\"allowed\":true,\"patch\":\"W3sib3AiOiJhZGQiLCJwYXRoIjoiL3NwZWMvY29udGFpbmVycy8wL2VudiIsInZhbHVlIjpbeyJuYW1lIjoiS1VCRSIsInZhbHVlIjoidHJ1ZSJ9XX0seyJvcCI6ImFkZCIsInBhdGgiOiIvc3BlYy90b2xlcmF0aW9ucy8tIiwidmFsdWUiOnsiZWZmZWN0IjoiTm9TY2hlZHVsZSIsImtleSI6ImFjbWUuY29tL2xpZmVzcGFuLXJlbWFpbmluZyIsIm9wZXJhdG9yIjoiRXhpc3RzIn19XQ==\",\"patchType\":\"JSONPatch\"}}" uri="/mutate-pods?timeout=2s"
time="2023-10-10T12:26:21Z" level=debug msg="received validation request" uri="/validate-pods?timeout=2s"
time="2023-10-10T12:26:21Z" level=info msg="delete me" pod_name=offensive-pod
time="2023-10-10T12:26:21Z" level=debug msg="sending response" uri="/validate-pods?timeout=2s"
time="2023-10-10T12:26:21Z" level=debug msg="{\"kind\":\"AdmissionReview\",\"apiVersion\":\"admission.k8s.io/v1\",\"response\":{\"uid\":\"b6809378-16f7-4d3a-bf01-ffed66ec2de9\",\"allowed\":false,\"status\":{\"metadata\":{},\"message\":\"pod name contains \\\"offensive\\\"\",\"code\":403}}}" uri="/validate-pods?timeout=2s"
```