# E2E test networking

This document explains how the e2e test environment wires up networking. The
suite used to rely on [metal3-dev-env](https://github.com/metal3-io/metal3-dev-env)
to build all of the host bridges, NAT, firewall rules and the provisioning
network implicitly. The e2e tests now create that environment explicitly using
the [vbmctl](https://github.com/metal3-io/baremetal-operator) CLI, split between
a declarative topology (`test/e2e/config/vbmctl.yaml.tmpl`) and host-side glue in
the shell scripts (`hack/setup-bml.sh`).

## The two-world problem

The management cluster runs in **kind** (Docker networking), while the "bare
metal" hosts are **libvirt VMs** (libvirt networking). These are two separate L2
domains that must talk to each other: Ironic (inside kind) has to reach both the
BMC emulator and the VMs.

```mermaid
flowchart TB
    subgraph host["CI Host"]
        subgraph docker["Docker world"]
            kind["kind mgmt cluster<br/>(Ironic, CAPM3, BMO)<br/>on kind-bridge<br/>172.19.0.0/16"]
        end
        subgraph libvirt["libvirt world"]
            sushy["sushy-tools BMC<br/>172.22.0.1:8000"]
            imgsrv["image server<br/>:80"]
            vms["VMs: node-0..N<br/>(virtual bare metal)"]
        end
    end
    kind -. "must reach BMC + VMs<br/>across the boundary" .-> libvirt

    classDef d fill:#bbdefb,stroke:#0d47a1,stroke-width:2px,color:#0d47a1;
    classDef l fill:#e1bee7,stroke:#4a148c,stroke-width:2px,color:#4a148c;
    class kind d;
    class sushy,imgsrv,vms l;
```

## Network topology

`vbmctl` creates two libvirt networks/bridges. Each VM is dual-homed (one NIC per
network with deterministic MAC addresses), and the two libvirt bridges are
stitched into the kind Docker bridge using veth pairs.

| Network            | Bridge     | Address          | Role                         |
| ------------------ | ---------- | ---------------- | ---------------------------- |
| `provisioning-e2e` | `provisioning` | `172.22.0.1/24`  | PXE / Ironic provisioning    |
| `external-e2e`     | `external` | `192.168.111.1/24` | tenant / API-server traffic |

```mermaid
flowchart TB
    subgraph HOST["🖥️ CI Host (bare metal / VM) — Linux kernel networking + nftables"]
        direction TB

        subgraph DOCKER["🐳 Docker world"]
            KB(("kind-bridge<br/>172.19.0.0/16<br/>(pinned)"))
            KIND["kind node (container)<br/>Ironic · CAPM3 · BMO<br/>+ DHCP/image serving"]
            KIND --- KB
        end

        FW{{"nftables filter table<br/>DOCKER-USER chain<br/>(policy drop → explicit ACCEPT<br/>for provisioning &amp; external bridges)"}}

        subgraph LIBVIRT["🧬 libvirt world"]
            direction TB
            PB(("bridge: provisioning<br/>net: provisioning-e2e<br/>172.22.0.1/24"))
            EB(("bridge: external<br/>net: external-e2e<br/>192.168.111.1/24"))

            SUSHY["sushy-tools BMC (container)<br/>172.22.0.1:8000<br/>Redfish/virtualmedia"]
            IMG["image server (container)<br/>:80"]

            subgraph VMS["Virtual bare metal VMs"]
                direction TB
                N0["node-0<br/>NIC1 prov 00:60:2f:31:81:01<br/>NIC2 ext  00:60:2f:32:81:01<br/>ext IP 192.168.111.20"]
                N1["node-1<br/>…:02 · ext .21"]
                NN["node-N<br/>…:0N · ext .2N"]
            end

            SUSHY --- PB
            IMG --- PB
            PB --- N0
            EB --- N0
            PB --- N1
            EB --- N1
            PB --- NN
            EB --- NN
        end

        %% veth pairs stitching libvirt bridges into the Docker bridge
        PB <==>|"veth pair<br/>metalend ↔ kindend"| KB
        EB <==>|"veth pair<br/>extmetal ↔ extkind"| KB

        %% firewall gate governing cross-boundary forwarding
        KB -.->|"forwarded traffic<br/>passes filter"| FW
        FW -.-> PB
        FW -.-> EB

        NIC["host uplink / NAT<br/>→ internet"]
        PB -. NAT .-> NIC
        EB -. NAT .-> NIC
    end

    classDef docker fill:#bbdefb,stroke:#0d47a1,stroke-width:2px,color:#0d47a1;
    classDef net fill:#ffe0b2,stroke:#e65100,stroke-width:2px,color:#bf360c;
    classDef vm fill:#c8e6c9,stroke:#1b5e20,stroke-width:2px,color:#1b5e20;
    classDef svc fill:#d1c4e9,stroke:#4527a0,stroke-width:2px,color:#311b92;
    classDef fw fill:#ffcdd2,stroke:#b71c1c,stroke-width:2px,color:#b71c1c;

    class KB,KIND docker;
    class PB,EB,NIC net;
    class N0,N1,NN vm;
    class SUSHY,IMG svc;
    class FW fw;
```

Pinning the kind Docker network to a fixed subnet (`172.19.0.0/16`) gives the
veth pairs a stable, known target; previously kind picked an
arbitrary Docker network.

## End-to-end provisioning path

Putting it together, this is how Ironic reaches a virtual bare metal host to
provision it:

```mermaid
flowchart LR
    I["Ironic<br/>(in kind)"]
    I -->|"1 · BMC/Redfish<br/>via veth+DOCKER-USER"| S["sushy-tools<br/>172.22.0.1:8000"]
    S -->|"2 · power/boot control"| VM["node-i VM"]
    VM -->|"3 · PXE/iPXE on<br/>provisioning net"| PB(("provisioning<br/>172.22.0.1"))
    I -->|"4 · serve image<br/>DHCP 172.22.0.10-100"| VM
    VM -->|"5 · deployed, API traffic"| EB(("external<br/>192.168.111.x"))

    classDef net fill:#ffe0b2,stroke:#e65100,stroke-width:2px,color:#bf360c;
    classDef svc fill:#bbdefb,stroke:#0d47a1,stroke-width:2px,color:#0d47a1;
    classDef vm fill:#c8e6c9,stroke:#1b5e20,stroke-width:2px,color:#1b5e20;
    class PB,EB net;
    class I,S svc;
    class VM vm;
```
