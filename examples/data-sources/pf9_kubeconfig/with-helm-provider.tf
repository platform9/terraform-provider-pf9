terraform {
  required_providers {
    pf9 = {
      source  = "platform9/pf9"
    }
    kubernetes = {
      source = "hashicorp/kubernetes"
      version = ">= 2.7.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = ">= 2.0.1"
    }
  }
}

provider "pf9" {}

variable "cluster_name" {
  type = string
  default = "mycluster"
}

data "pf9_clusters" "example" {
  filters = [ {
    name   = "name"
    values = [var.cluster_name]
  } ]
}

data "pf9_kubeconfig" "example" {
  id = data.pf9_clusters.example.cluster_ids[0]
  authentication_method = "token"
}

provider "kubernetes" {
  host             = data.pf9_kubeconfig.kubeconfigs[0].endpoint
  token            = data.pf9_kubeconfig.kubeconfigs[0].token
  cluster_ca_certificate = base64decode(
    data.pf9_kubeconfig.kubeconfigs[0].cluster_ca_certificate
  )
}

provider "helm" {
  kubernetes {
    host  = data.pf9_kubeconfig.example.kubeconfigs[0].host
    token = data.pf9_kubeconfig.example.kubeconfigs[0].token
  }
}

resource "helm_release" "nginx_ingress" {
  name      = "nginx-ingress-controller"
  namespace = kubernetes_namespace.test.metadata.0.name

  repository = "https://charts.bitnami.com/bitnami"
  chart      = "nginx-ingress-controller"

  set {
    name  = "service.type"
    value = "LoadBalancer"
  }
  set {
    name  = "service.annotations.service\\.beta\\.kubernetes\\.io/do-loadbalancer-name"
    value = format("%s-nginx-ingress", var.cluster_name)
  }
}

resource "kubernetes_ingress_v1" "test_ingress" {
  wait_for_load_balancer = true
  metadata {
    name      = "test-ingress"
    namespace = kubernetes_namespace.test.metadata.0.name
    annotations = {
      "kubernetes.io/ingress.class"          = "nginx"
      "ingress.kubernetes.io/rewrite-target" = "/"
    }
  }

  spec {
    rule {
      http {
        path {
          backend {
            service {
              name = kubernetes_service.test.metadata.0.name
              port {
                number = 5678
              }
            }
          }

          path = "/test"
        }
      }
    }
  }
}