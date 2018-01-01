#!/bin/sh
set -e

export KUBECONFIG=/tmp/kubensx-spec-kubeconfig
cat $(dirname "$0")/kubeconfig.envsubst.yml | MINIKUBE_IP=$(minikube ip) envsubst > $KUBECONFIG

go build

set -x

./kubensx --debug ls -u
./kubensx --debug ls -c

./kubensx --debug use minikube:minikube/default

./kubensx --debug current
./kubensx --debug current -u
./kubensx --debug current -c
./kubensx --debug current -n

# user:cluster/namespace
./kubensx --debug use -x ':/*'
./kubensx --debug use -x ':*/'
./kubensx --debug use -x '*:/'
# user:cluster
./kubensx --debug use -x :
./kubensx --debug use -x minikube:minikube
./kubensx --debug use -x kube:kube
# cluster/namespace
./kubensx --debug use -x /
./kubensx --debug use -x /default
./kubensx --debug use -x /def
# namespace
./kubensx --debug use -x ''
./kubensx --debug use -x default
./kubensx --debug use -x def

./kubensx --debug use -xu kube
# should yield 3 matches (us, us-west1 & us-east1)
./kubensx --debug use -xc 'us*'

# should yield 0 matches
./kubensx --debug use -x :us1/
# should yield 2 matches (us-west1 & us-east1)
./kubensx --debug use -x :us-/
# should yield 1 match (us, but not us-west1 & us-east1)
./kubensx --debug use -x :us/
# should yield 3 matches (us, us-west1 & us-east1)
./kubensx --debug use -x ':us*/'

# should yield 0 matches
./kubensx --debug use -xe :us1/
# should yield 0 matches
./kubensx --debug use -xe :us-/
# should yield 1 match (us, but not us-west1 & us-east1)
./kubensx --debug use -xe :us/

# should yield 2 matches
./kubensx --debug use -xz :us1/
# should yield 2 matches
./kubensx --debug use -xz :us-/
# should yield 1 match (us, but not us-west1 & us-east1)
./kubensx --debug use -xz :us/

./kubensx --debug assoc minikube:minikube
./kubensx --debug assoc -l
# user
./kubensx --debug assoc -x minikube
./kubensx --debug assoc -x kube
# user:cluster
./kubensx --debug assoc -x minikube:minikube
./kubensx --debug assoc -x kube:kube
./kubensx --debug assoc -x 'kube:*'
./kubensx --debug assoc -x '*:kube'
# empty (not allowed)
./kubensx --debug assoc -x : && exit 1
./kubensx --debug assoc -x '*:' && exit 1
./kubensx --debug assoc -x ':*' && exit 1

# at this point minikube user is assoc[iated] with minikube cluster
# and so only one record should be printed
./kubensx --debug use -x '*:kube'
./kubensx --debug use -x --ignore-assoc '*:kube'

echo done
