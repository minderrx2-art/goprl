terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

provider "google" {
  project = "go-url-shortener-486311"
  region  = "us-central1"
  zone    = "us-central1-a"
}

# 1. Create a Network
resource "google_compute_network" "vpc" {
  name = "go-app-network"
}

# 2. Open HTTP for everyone
resource "google_compute_firewall" "allow_http" {
  name    = "allow-http"
  network = google_compute_network.vpc.name

  allow {
    protocol = "tcp"
    ports    = ["80"]
  }

  source_ranges = ["0.0.0.0/0"]
}

# 3. Open SSH ONLY for my IP
resource "google_compute_firewall" "allow_ssh" {
  name    = "allow-ssh"
  network = google_compute_network.vpc.name

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["45.148.56.90/32"]
}

# 4. The Server
resource "google_compute_instance" "vm" {
  name         = "go-url-shortener"
  machine_type = "e2-micro"
  tags         = ["http-server"]

  boot_disk {
    initialize_params { image = "ubuntu-os-cloud/ubuntu-2204-lts" }
  }

  network_interface {
    network = google_compute_network.vpc.name
    access_config {} # Gives it a public IP
  }

  metadata_startup_script = "curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh"
}

# Return IP of the cloud server
output "ip" {
  value = google_compute_instance.vm.network_interface[0].access_config[0].nat_ip
}
