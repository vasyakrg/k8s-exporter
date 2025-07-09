# k8s-exporter

Prometheus exporter для сбора информации о нодах из нескольких Kubernetes кластеров.

## Запуск

1. Установите зависимости:

   ```bash
   go mod tidy
   ```

2. Сформируйте `kube` (подразумевается, что файлики с yaml контекстами лежат в ~/.kube)

```bash
kube config view -o yaml --raw > kube
```

3. Запустите экспортер:

   ```bash
   go run main.go --config kube
   ```

4. Метрики будут доступны на <http://localhost:8080/metrics>

## Экспортируемые метрики

- `k8s_node_info`
- `k8s_node_condition`
- `k8s_node_capacity`
- `k8s_node_allocatable`

## Контейнеризация

Сборка контейнера:

```bash
docker build -t k8s-exporter:latest .
```

Запуск:

```bash
docker run -p 8080:8080 -v $(pwd)/kube:/app/kube k8s-exporter:latest
```

5. Запуск через helm

```bash
helm upgrade --install --create-namespace -n k8s-exporter k8s-exporter charts/k8s-exporter
```
