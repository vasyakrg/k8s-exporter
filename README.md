# k8s-exporter

Prometheus exporter для сбора информации о нодах из нескольких Kubernetes кластеров.

## Запуск

1. Установите зависимости:
   ```sh
   go mod tidy
   ```
2. Отредактируйте `kube`, указав пути к kubeconfig и контексты для каждого кластера.
3. Запустите экспортер:
   ```sh
   go run main.go --config kube
   ```
4. Метрики будут доступны на http://localhost:8080/metrics

## Экспортируемые метрики
- `k8s_node_info`
- `k8s_node_condition`
- `k8s_node_capacity`
- `k8s_node_allocatable`

## Контейнеризация

Сборка контейнера:
```sh
docker build -t k8s-exporter:latest .
```

Запуск:
```sh
docker run -p 8080:8080 -v $(pwd)/kube:/app/kube k8s-exporter:latest
```
