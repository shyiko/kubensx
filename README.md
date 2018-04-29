# kubensx ![Latest Version](https://img.shields.io/badge/latest-0.2.0-blue.svg) [![Build Status](https://travis-ci.org/shyiko/kubensx.svg?branch=master)](https://travis-ci.org/shyiko/kubensx)

Simpler Cluster/User/Namespace switching for Kubernetes  
(featuring interactive mode and wildcard/fuzzy matching (among other things)).

[![asciicast](https://asciinema.org/a/wtn1L6Tq4wavQcKbIn45lDiLe.png)](https://asciinema.org/a/wtn1L6Tq4wavQcKbIn45lDiLe)  

In short, instead of
```sh
kubectl config get-clusters
kubectl config view -o=go-template --template=$'{{range $u := .users}}{{$u.name}}\n{{end}}'
kubectl get --cluster=<cluster> --user=<user> namespaces
kubectl config set-context <context_name> --cluster=<cluster> --user=<user> --namespace=<namespace>
kubectl config use-context <context_name>
```
context can be changed with `kubensx use user:cluster/namespace` or simply `kubensx use` (interactive).

## Installation

#### macOS / Linux

```sh
curl -sSL https://github.com/shyiko/kubensx/releases/download/0.2.0/kubensx-0.2.0-$(
    bash -c '[[ $OSTYPE == darwin* ]] && echo darwin || echo linux'
  )-amd64 -o kubensx && chmod a+x kubensx && sudo mv kubensx /usr/local/bin/
```

Verify PGP signature (optional but recommended): 

```
curl -sSL https://github.com/shyiko/kubensx/releases/download/0.2.0/kubensx-0.2.0-$(
    bash -c '[[ $OSTYPE == darwin* ]] && echo darwin || echo linux'
  )-amd64.asc -o kubensx.asc
curl -sS https://keybase.io/shyiko/pgp_keys.asc | gpg --import
gpg --verify kubensx.asc /usr/local/bin/kubensx
```  

> macOS: `gpg` can be installed with `brew install gnupg`

#### Windows

Download executable from the [Releases](https://github.com/shyiko/kubensx/releases) page.

## Usage

```sh
# change <user>:<cluster>/<namespace> (interactive)
$ kubensx use
# change <namespace> only (interactive)
$ kubensx use -n

# switch to <user>:<cluster>/<namespace> 
$ kubensx use minikube:minikube/default
# switch to a different <namespace> within current <cluster> (<user> stays the same)
$ kubensx use kube-public

# context matching is wildcard-ish by default, which means you don't have to type the whole thing
# if there are two or more options available - you'll be asked to select one
$ kubensx use west/def
Switched to account@possibly-gmail.com:us-west1/default
# prefer fuzzy?
$ kubensx use -z us1/dfl
Switched to account@possibly-gmail.com:us-west1/default

# switch to previous context
$ kubensx use -

# print current context
$ kubensx current
minikube:minikube/default

# list <user>s
$ kubensx ls -u
# list <cluster>s
$ kubensx ls -c
# list <namespace>s (inside current <cluster>)
$ kubensx ls -n
```

> (for more information see `kubensx --help`)

#### User &lt;-&gt; Cluster assoc[iation] 

By default, any `<user>` can be used with any `<cluster>`.  
If you want to restrict (assoc[iate]) certain user(s) to some of the clusters use 
```sh
$ kubensx assoc
```

For example: if you have a "minikube" user which you only use in the context of local "minikube" cluster, 
you may want to `kubensx assoc minikube:minikube` (`<user>:<cluster>`) so that "minikube" 
wouldn't be shown among the users for any cluster other than "minikube" (when `kubesec use`ing).

#### Access Control

If a user is not allowed to list namespaces, you can either provide a list of namespaces known to that user with `ns-list`

```sh
$ kubensx ns-list
# make default, kube-system and kube-public namespaces known to current user in us-west1 cluster
$ kubensx ns-list us-west1/{default,kube-system,kube-public}
$ kubensx use west/def
Switched to account@possibly-gmail.com:us-west1/default
```
or use --force(-f) to suppress namespace validation (namespace will have to be provided --exact|ly)

```sh
$ kubensx use west/default --force
Switched to account@possibly-gmail.com:us-west1/default
```

#### <kbd>Tab</kbd> completion

```sh
# bash
$ source <(kubensx completion bash)

# zsh
$ source <(kubensx completion zsh)
```

## Development

> PREREQUISITE: [go1.9+](https://golang.org/dl/).

```sh
git clone https://github.com/shyiko/kubensx $GOPATH/src/github.com/shyiko/kubensx
cd $GOPATH/src/github.com/shyiko/kubensx
make fetch

go run kubensx.go
```

## Legal

All code, unless specified otherwise, is licensed under the [MIT](https://opensource.org/licenses/MIT) license.  
Copyright (c) 2018 Stanley Shyiko.
