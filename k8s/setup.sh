minikube start --driver=docker --cpus=4
minikube addons enable metrics-server
kubectl create namespace argocd 
kubectl create namespace order-inventory

kubectl apply -n argocd --server-side --force-conflicts -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl create -f 'https://strimzi.io/install/latest?namespace=order-inventory' -n order-inventory

kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d
echo
