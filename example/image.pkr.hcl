source "opennebula-image" "ubuntu" {
  opennebula_url    = "http://192.168.1.2:2633/RPC2"
  username          = "oneadmin"
  password          = "oneadmin"
  insecure          = true

  # Новые параметры для VMTemplateConfig
  vm_name           = "test"
  vm_cpu            = 4
  vm_cpu_model      = "host-passthrough"
  vm_memory         = 4096
  vm_vcpu           = 4

  # Один диск
  image {
    name = "tmp"
    clone_from_image = "golden-image"
    datastore_id     = 1
    persistent       = true
  }

  # Массив сетевых интерфейсов
  vm_nics {
    network = "public_net"
  }

  vm_graphics_type  = "vnc"
  vm_graphics_keymap = "en-us"
  vm_graphics_listen = "0.0.0.0"
  vm_user_data      = ""
  vm_os_arch        = "x86_64"

  ssh_handshake_attempts = 50
  ssh_username = "ubuntu"
  ssh_password = "ubuntu"
  ssh_port = 22
  ssh_pty = true
  snapshot {
    name = "opensearch"
    datastore_id   = 1
  }
}

build {
  name = "opennebula"
  sources = ["source.opennebula-image.ubuntu"]
  provisioner "shell" {
    inline = [
      "while [ ! -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for Cloud-Init...'; sleep 1; done"
    ]
  }

  provisioner "shell" {
    execute_command = "chmod +x {{ .Path }}; {{ .Vars }} sudo -E sh '{{ .Path }}'"
    inline = [
      "apt update && apt upgrade -y && apt dist-upgrade -y && apt autoremove -y && apt autoclean -y"
    ]
  }

}

packer {
  required_version = ">= 1.9.0"
}
