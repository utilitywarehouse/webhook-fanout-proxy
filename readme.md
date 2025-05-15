# webhook-fanout-proxy

webhook-fanout-proxy is a service to forward webhook events to given targets.

`webhook-fanout-proxy` is used to fanout github webhook events to all required
pods running `git-mirror` in kubernetes.

## Webhook Config

```yaml
webhooks:
  # The URL path on which request are received.
  - path: /webhook/example
    # allowed method for the HTTP request.
    method: POST
    # Specifies the HTTP response that will be returned on valid requests.
    response:
      code: 200
      headers:
        - name: Access-Control-Allow-Origin
          value: "*"
      message: "ok"
    # list of targets where received payload will be sent
    targets:
      - http://127.0.0.1:8080/webhook

  - path: /github
    method: POST
    response:
      code: 204
    targets:
      - http://prometheus-shared-0.prometheus-git-mirror:9001/github-webhook
      - http://prometheus-shared-1.prometheus-git-mirror:9001/github-webhook
      - http://thanos-shared-0.thanos-git-mirror:9001/github-webhook
      - http://thanos-shared-1.thanos-git-mirror:9001/github-webhook
```

Note:

- webhooks will use same port as metrics server
