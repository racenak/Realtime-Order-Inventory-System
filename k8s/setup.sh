minikube start --driver=docker
kubectl create namespace argocd 
kubectl create namespace order-inventory

kubectl apply -n argocd --server-side --force-conflicts -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

kubectl create -f 'https://strimzi.io/install/latest?namespace=order-inventory' -n order-inventory