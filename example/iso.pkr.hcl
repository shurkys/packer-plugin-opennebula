source "opennebula-iso" "ubuntu" {
  opennebula_url    = "http://192.168.1.2:2633/RPC2"
  username          = "oneadmin"
  password          = "oneadmin"
  insecure          = true

  eject_iso = true
  eject_iso_delay = "20m"
  # Новые параметры для VMTemplateConfig
  vm_name           = "test"
  vm_cpu            = 8
  vm_cpu_model      = "host-passthrough"
  vm_memory         = 8192
  vm_vcpu           = 8

  # Два диска, первый iso
  image {
    name = "ubuntu-install"
  }
  image {
    name = "test"
    size        = 10240  // Размер нового диска
    persistent = true
    datastore_id   = 1
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
  vm_os_boot        = "disk0"

  boot_command= [
    "c<wait>",
    "linux /casper/vmlinuz",
    " autoinstall ds=\"nocloud-net;seedfrom=http://{{ .HTTPIP }}:{{ .HTTPPort }}/\"",
    " ip=192.168.1.201::192.168.1.1:255.255.255.0::::192.168.1.1",
    " --- <enter><wait>",
    "initrd /casper/initrd",
    "<enter><wait>",
    "boot",
    "<enter>"
  ]

  boot_key_interval="75ms"
  http_directory= "/home/shurik/packer-plugin-opennebula/example/http"
  boot_wait      = "10s"
  vm_vnc_password = ""

  ssh_handshake_attempts = 50
  ssh_username = "ubuntu"
  ssh_password = "ubuntu"
  ssh_port = 22
  ssh_pty = true
  snapshot {
    name = "golden-image"
    datastore_id   = 1
  }
}

build {
  name = "opennebula-iso"
  sources = ["source.opennebula-iso.ubuntu"]
  provisioner "shell" {
    inline = [
      "while [ ! -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for Cloud-Init...'; sleep 5; done"
    ]
  }

  provisioner "shell" {
    execute_command = "chmod +x {{ .Path }}; {{ .Vars }} sudo -E sh '{{ .Path }}'"
    inline = [
      "while [ ! -f /var/lib/apt/lists/lock ]; do echo 'Waiting for Cloud-Init...'; sleep 5; done &&  apt update && apt upgrade -y && apt dist-upgrade -y && apt autoremove -y && apt autoclean -y"
    ]
  }
}

packer {
  required_version = ">= 1.9.0"
}
