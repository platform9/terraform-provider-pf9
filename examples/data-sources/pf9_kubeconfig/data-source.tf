data "pf9_kubeconfig" "example" {
  id = "8ab77ab1-cc35-4976-adc7-8f6683fd348b"
  authentication_method = "token"
}

output "example" {
  value = data.pf9_kubeconfig.example.raw
}

variable "cluster_name" {
  type = string
  default = "mycluster"
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