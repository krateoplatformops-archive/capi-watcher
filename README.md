# Cluste API watcher

## How to install

```shell
$ helm repo add krateo https://charts.krateo.io
$ helm repo update krateo
$ helm install capi-watcher krateo/capi-watcher --namespace krateo-system --create-namespace
```

# Env Vars

| Name                     | Description                   | Default Value     |
|:-------------------------|:------------------------------|:------------------|
| CAPI_WATCHER_DEBUG       | dump verbose output           | false             |
| CAPI_WATCHER_NAMESPACE   | namespace to list and watch   | all namespaces    |
