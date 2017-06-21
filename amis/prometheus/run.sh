#!/bin/bash

PROMETHEUS_URL=192.168.99.100:3000
PROMETHEUS_USER=admin
PROMETHEUS_PW=admin

echo "run prometheus"

docker-compose up -d

echo ""
sleep 3
echo "create datasource"

curl -X POST -d @grafana/datasource.json "http://$PROMETHEUS_USER:$PROMETHEUS_PW@$PROMETHEUS_URL/api/datasources" \
     -H "Accept: application/json" \
     -H "Content-Type:application/json"

echo ""
echo "create dashboard "

curl -X POST -d @grafana/dashboard.json "http://$PROMETHEUS_USER:$PROMETHEUS_PW@$PROMETHEUS_URL/api/dashboards/db" \
     -H "Accept: application/json" \
     -H "Content-Type:application/json"

echo ""
