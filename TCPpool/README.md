## Build images

```

sudo docker build ./Server -t localhost:32000/cpools:test
sudo docker build ./Client -t localhost:32000/cpoolc:test

```
## Push images to registry

## Create service

```

kubectl apply -f mqtt_broker.yaml 
kubectl apply -f ./Server/CpoolS.yaml
kubectl apply -f ./Client/CpoolC.yaml

```

## Get TPS

```

sudo apt update -y && sudo apt install mosquitto-clients -y
mosquitto_sub -h localhost -p 31883 -t sensor/test

```
