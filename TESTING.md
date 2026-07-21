# Manual test roteiro

Passo a passo para validar, num host real com KVM, as mudanças implementadas no `vmctl` (o binário Go que substitui os antigos `debian-vm-setup.sh`, `debian-vm-cleanup.sh` e `debian-vm-backup.sh`), incluindo `vmctl list`/`info` e os metadados consolidados (`meta.json`).

Nenhum destes testes foi (ou pôde ser) executado em CI/sandbox — todos exigem uma VM real. Rode-os na ordem sugerida; várias seções reaproveitam a mesma VM (`test-01`) para economizar tempo de boot.

**Status da última rodada completa (2026-07-20/21):** a maior parte do roteiro foi executada contra um host real, o que revelou e corrigiu 7 bugs reais (ver `design.md` e o histórico de commits). Os seguintes pontos ficaram **sem cobertura** nessa rodada e continuam pendentes de validação:
- **`--bridge` (modo bridged)**, seções 5 e 9 — pulado por falta de interface cabeada disponível no host de teste.
- Disparo real do watchdog (`echo c > /proc/sysrq-trigger`, seção 6) — não executado.
- Teste do motd via logout/login na sessão do host (seção 5) — não executado.
- Seção 7 (`on_crash=restart` via `kill -9`) — executado, mas terminou como achado **não resolvido** (a VM não reiniciou sozinha); ver a nota ⚠️ na seção e `design.md`'s Open Questions. Não tratar como "passou".
- **Seção 0 (`vmctl doctor`)** — validada (2026-07-21) contra um host real, incluindo o ciclo completo `--fix`/`--unfix`/`--fix`; achou e corrigiu 1 bug real (`Fix` não redefinia a rede `default` depois de `Unfix`, ver a nota ⚠️ na seção 0.5).
- **Seção 11 (rename da CLI: `create`/`start`/`stop`/`reboot`/`delete`/`info`/`snapshot`/`backup`)** — nova, ainda sem nenhuma rodada real executada. Pendente de validação.

## Pré-requisitos (uma vez)

```sh
cd vmctl
go build -o vmctl ./cmd/vmctl
./vmctl --help   # confirma que compilou e lista os subcomandos
cd ..
```

Para não repetir o caminho, aponte uma variável pro binário:
```sh
VMCTL=./vmctl/vmctl
$VMCTL --help
```

Confirme suporte a virtualização e que você não está como root:
```sh
egrep -c '(vmx|svm)' /proc/cpuinfo   # deve ser > 0
whoami                                # não deve ser root
```

---

## 0. `vmctl doctor` (pré-requisitos de host)

`vmctl create` não instala/configura mais nada no host — só verifica. `vmctl doctor` é quem instala (`--fix`) ou remove (`--unfix`). Este roteiro precisa de `sudo` interativo de verdade (não roda em sandbox sem TTY) — execute cada bloco você mesmo num terminal normal e reporte o resultado (saída + código de saída, `echo $?`) de volta.

Cada passo tem um nível de risco. Pare e me reporte depois de cada bloco antes de ir pro próximo, principalmente ao entrar no nível 🔴.

### 0.1 — 🟢 Estado atual (somente leitura)

```sh
$VMCTL doctor; echo "exit: $?"
```
**Esperado:** uma linha `[OK]`/`[MISSING]` por item (pacotes, grupo libvirt/kvm, libvirtd, rede `default`, ACL do `libvirt-qemu`). Se este host já foi usado antes, é normal já estar tudo `[OK]` e `exit: 0` — não muda nada de qualquer forma.

**Reporte:** cole a saída completa + o exit code.

### 0.2 — 🟡 `--fix` idempotente (seguro se o host já está `[OK]`; instala coisas reais se não estiver)

```sh
$VMCTL doctor --fix; echo "exit: $?"
$VMCTL doctor; echo "exit: $?"
```
**Esperado:** `--fix` roda `apt update`/`install`, `usermod`, `systemctl enable --now libvirtd`, ativa a rede `default` e concede a ACL — cada passo é um no-op se já estiver feito. O `doctor` seguinte deve mostrar tudo `[OK]` e `exit: 0`.

Se `--fix` acabou de adicionar seu usuário aos grupos `libvirt`/`kvm` pela primeira vez: **faça logout/login** antes de continuar (a própria mensagem de erro deve avisar disso caso a sessão atual ainda não tenha o grupo ativo — rode `id -nG` pra conferir).

**Reporte:** cole as duas saídas + exit codes. Se algum passo do `--fix` falhar, cole o erro específico.

### 0.3 — 🟢 `vmctl create` sem sudo (cria uma VM descartável)

```sh
$VMCTL create --name=doctor-test-01; echo "exit: $?"
```
**Esperado:** a única menção a pré-requisitos é uma linha tipo `OK: host prerequisites present.` — **nenhuma** chamada de `apt`/`usermod`/`systemctl`/`setfacl` deve aparecer na saída (procure por essas strings). O resto é a criação normal da VM.

Depois, limpe:
```sh
$VMCTL delete --name=doctor-test-01 --vm-only; echo "exit: $?"
```

**Reporte:** confirme se apareceu ou não alguma chamada de instalação na saída do `setup` (cole o trecho relevante).

### 0.4 — 🟡 Fail-fast com um pré-requisito faltando (remove e reinstala um pacote não-crítico)

```sh
sudo apt remove -y genisoimage
$VMCTL create --name=doctor-test-02; echo "exit: $?"
```
**Esperado:** falha IMEDIATA, antes de qualquer trabalho de criação de VM, citando `genisoimage` especificamente e apontando pra `vmctl doctor --fix`.

Restaure:
```sh
sudo apt install -y genisoimage
$VMCTL doctor; echo "exit: $?"   # deve voltar a tudo [OK]
```

**Reporte:** cole a mensagem de erro do `setup` e a confirmação de que voltou a `[OK]` depois.

### 0.5 — 🔴 `--unfix` (remove KVM/libvirt/grupos/rede de verdade deste host — opcional)

Só faça isso se estiver de acordo em desinstalar KVM/libvirt temporariamente. Pule pro final se preferir considerar a validação encerrada em 0.4.

```sh
virsh list --all   # confirme se está vazio; se não estiver, anote quais VMs existem
```

**Teste da recusa (crie uma VM de propósito pra isso):**
```sh
$VMCTL create --name=doctor-test-03
$VMCTL doctor --unfix; echo "exit: $?"
```
**Esperado:** recusa (exit != 0), citando `doctor-test-03` e apontando pra removê-la primeiro com `vmctl delete --vm-only`.

**Teste da remoção de verdade:**
```sh
$VMCTL delete --name=doctor-test-03 --vm-only
virsh list --all   # confirme vazio agora

$VMCTL doctor --unfix; echo "exit: $?"
$VMCTL doctor; echo "exit: $?"
```
**Esperado:** `--unfix` roda sem recusar, remove a rede `default`, purga os pacotes, remove os grupos e revoga a ACL. O `doctor` seguinte deve mostrar tudo `[MISSING]`.

**Re-provisionamento (fecha o ciclo):**
```sh
$VMCTL doctor --fix; echo "exit: $?"
$VMCTL doctor; echo "exit: $?"   # deve voltar a tudo [OK]
```

⚠️ **Achado de teste real (2026-07-21)**: rodando esse ciclo completo contra um host real, `--fix` falhou ao tentar re-provisionar depois do `--unfix`, porque `--unfix` roda `virsh net-undefine default` (remove a definição inteira, não só desativa), enquanto `ensureNATNetworkReady` (dentro do `Fix`) só sabia ativar/autostart uma rede já definida, não redefini-la do zero — o próprio round-trip `--unfix` → `--fix` nunca tinha sido exercitado antes desta rodada. Corrigido: `Fix` agora roda `virsh net-define /usr/share/libvirt/networks/default.xml` quando a rede não existe, antes de tentar ativá-la/autostart. Recompile (`go build -o vmctl ./cmd/vmctl`) antes de repetir este bloco.

**Reporte:** cole as quatro saídas principais (recusa, remoção, `[MISSING]`, `[OK]` final).

---

## 1. `--admin-password` (sudo com senha)

```sh
# Cria a VM com senha de sudo
$VMCTL create --name=test-01 --admin-password
```

**Esperado:**
- Output mostra a senha gerada uma vez, e o caminho `~/vms/test-01/admin-password` (arquivo com `chmod 600`).
- Nota final menciona `virsh console test-01` como escape hatch.

```sh
# Confirma permissões do arquivo de senha
stat -c '%a' ~/vms/test-01/admin-password   # deve ser 600
cat ~/vms/test-01/meta.json                 # deve conter "admin_sudo_policy": "password-required"

# Confirma que SSH continua só-chave (não deve pedir senha)
ssh -o PasswordAuthentication=no admin@$(virsh domifaddr test-01 | awk '/ipv4/{print $4}' | cut -d/ -f1)

# Dentro da VM, confirma que sudo agora pede senha
ssh admin@<VM_IP> 'sudo -n true' # deve FALHAR (sudo exige senha, -n = non-interactive)
ssh -t admin@<VM_IP> 'sudo whoami'  # deve pedir a senha do arquivo admin-password
```

**Teste de rerun-mismatch:**
```sh
$VMCTL create --name=test-01   # sem --admin-password, mesma VM
```
Esperado: `WARNING: VM 'test-01' already exists with password-required sudo, but this run did not request --admin-password.` — e a VM continua com sudo exigindo senha (não muda nada).

**Teste do escape hatch:**
```sh
virsh console test-01   # Ctrl+] para sair
# dentro do console: passwd admin
```

---

## 2. `--no-auto-updates` (unattended-upgrades, default ON)

```sh
# A VM test-01 já foi criada SEM --no-auto-updates (default), então já deve ter isso ativo:
ssh admin@<VM_IP> 'dpkg -l unattended-upgrades'
ssh admin@<VM_IP> 'systemctl is-enabled apt-daily-upgrade.timer apt-daily.timer'
ssh admin@<VM_IP> 'cat /etc/apt/apt.conf.d/51unattended-upgrades-security-only'
ssh admin@<VM_IP> 'unattended-upgrades --dry-run --debug 2>&1 | grep -i origin'
```

**Esperado:** pacote instalado, timers habilitados, arquivo de override presente com `${distro_id}:${distro_codename}-security` (já resolvido pelo unattended-upgrades, não deve aparecer literal `${distro_id}` no dry-run).

**Teste do opt-out** (VM nova, para não conflitar com a anterior):
```sh
$VMCTL create --name=test-02 --no-auto-updates
ssh admin@<VM_IP_02> 'cat /etc/apt/apt.conf.d/51unattended-upgrades-security-only'
```
**Esperado:** `No such file or directory` — esse arquivo só é escrito pelo cloud-init quando `--no-auto-updates` NÃO é passado. **Não** confie em `dpkg -l unattended-upgrades` pra esse teste: a imagem cloud oficial do Debian 12 já vem com o pacote pré-instalado independente de qualquer flag, então ele aparece instalado nos dois casos — o que muda é só o arquivo de override.

---

## 3. Guest firewall (`--allow-port`, `--no-guest-firewall`) + fail2ban

```sh
# test-01 já tem o firewall padrão (default-deny). test-01 tem sudo com senha (teste 1) — usa -t.
ssh -t admin@<VM_IP> 'sudo ufw status'
ssh -t admin@<VM_IP> 'sudo fail2ban-client status sshd'
```
**Esperado:** `Status: active`, `22/tcp ALLOW`, política default `deny (incoming), allow (outgoing)`. Jail `sshd` ativo.

**Teste de `--allow-port` + `--forward` (VM nova):**
```sh
$VMCTL create --name=test-03 --allow-port=8080 --forward=9000:80
ssh admin@<VM_IP_03> 'sudo ufw status numbered'
```
**Esperado:** portas 22, 80 (derivada do `--forward`) e 8080 (`--allow-port`) todas `ALLOW`.

**Teste do warning de `--forward` tardio:**
```sh
$VMCTL create --name=test-03 --forward=9001:81
```
**Esperado:** a regra DNAT/FORWARD é aplicada no host normalmente, MAS aparece:
```
WARNING: this VM's guest firewall (ufw) was enabled at creation, but its allow
         list can't be updated by rerunning this...
           ssh admin@<VM_IP_03> sudo ufw allow 81/tcp
```
Confirme que a porta 81 NÃO está liberada dentro do guest até você rodar o comando sugerido manualmente.

**Teste do opt-out:**
```sh
$VMCTL create --name=test-04 --no-guest-firewall --allow-port=9999
ssh admin@<VM_IP_04> 'which ufw'   # não deve existir
ssh admin@<VM_IP_04> 'sudo fail2ban-client status sshd'   # deve continuar ativo (não é afetado pela flag)
```

---

## 4. `--harden-host-firewall` (firewall do HOST, cuidado)

⚠️ Isso reconfigura o firewall da sua máquina física. Rode num host de teste, ou tenha acesso físico/console de emergência caso algo saia errado com SSH.

```sh
$VMCTL create --name=test-01 --harden-host-firewall
sudo ufw status verbose
```
**Esperado:** `ufw` ativo no HOST, regra `22/tcp ALLOW ... # self-hosting: host SSH baseline`, `DEFAULT_FORWARD_POLICY="ACCEPT"` em `/etc/default/ufw`.

**Confirma que o NAT/forward continuam funcionando:**
```sh
# não use curl aqui a menos que algo esteja de fato escutando na porta da VM
# (a imagem cloud crua não tem nenhum servidor); confirme a regra em si:
sudo iptables -t nat -L PREROUTING -n --line-numbers | grep <HOST_PORT>   # ex: a porta do --forward do teste 3
ssh admin@$(virsh domifaddr test-01 | awk '/ipv4/{print $4}' | cut -d/ -f1)   # NAT interno continua ok
```

**Teste de idempotência:**
```sh
$VMCTL create --name=test-01 --harden-host-firewall   # roda de novo
sudo ufw status numbered | grep -c "host SSH baseline"
```
**Esperado:** com IPv6 habilitado no ufw (padrão), `ufw allow ... comment "..."` cria uma regra v4 **e** uma v6 — então o valor esperado é **2**, estável entre reruns (o que importa é não crescer pra 4 na segunda vez, não ser exatamente 1).

**Teste de remoção (`vmctl delete`):**
```sh
$VMCTL delete --name=test-01 --vm-only
sudo ufw status | grep "host SSH baseline"   # deve AINDA existir (--vm-only preserva)

$VMCTL delete --name=test-01 --purge-all   # com nenhuma outra VM ativa
sudo ufw status | grep "host SSH baseline"   # não deve mais existir
grep DEFAULT_FORWARD_POLICY /etc/default/ufw   # deve voltar a "DROP"
which ufw   # ufw continua instalado
```

---

## 5. `--monitor` (uptime, logging, alerting)

```sh
$VMCTL create --name=test-05 --monitor
systemctl list-timers 'self-hosting-vm-uptime@*'
```
**Esperado:** timer `self-hosting-vm-uptime@test-05.timer` ativo.

**Teste de detecção de queda:**
```sh
virsh destroy test-05
sleep 130   # esperar pelo menos um ciclo do timer (~2min)
journalctl -t self-hosting-alert -n 5
```
**Esperado:** entrada `VM 'test-05' is DOWN`. Se havia sessão local ativa, `wall` deveria ter aparecido.

```sh
virsh start test-05
sleep 130
journalctl -t self-hosting-alert -n 5
```
**Esperado:** entrada `VM 'test-05' has RECOVERED`.

**Teste do motd** (⚠️ não executado na última rodada — pendente):
```sh
# faça logout/login no HOST (ou abra nova sessão SSH nele)
```
**Esperado:** banner de login mostra os alertas recentes.

**Teste de logging centralizado (NAT, sem --bridge):**
```sh
ssh admin@<VM_IP_05> 'logger "teste de log centralizado"'
sleep 5
sudo tail -5 /var/log/self-hosting-vms/test-05/messages.log
```
**Esperado:** a linha aparece no host.

**Teste com `--bridge` (logging deve ficar indisponível, uptime não)** (⚠️ não executado na última rodada — pulado por falta de interface cabeada disponível; pendente de validação):
```sh
$VMCTL create --name=test-06 --bridge=<sua-interface> --monitor
```
**Esperado:** nota explícita "Centralized logging is NOT available in bridged mode". `systemctl list-timers` ainda mostra o timer de test-06.

**Teste de cleanup:**
```sh
$VMCTL delete --name=test-05 --vm-only
systemctl is-enabled self-hosting-vm-uptime@test-05.timer   # deve estar "disabled"/inexistente
ls /var/log/self-hosting-vms/test-05/   # logs devem continuar lá

$VMCTL delete --name=test-05 --purge-all   # sem outras VMs
# deve perguntar "Delete accumulated VM logs...?" mesmo em modo não-interativo de outras etapas
```

---

## 6. `--watchdog`

```sh
$VMCTL create --name=test-07 --watchdog
virsh dumpxml test-07 | grep -A1 '<watchdog'
```
**Esperado:** `<watchdog model='i6300esb' action='reset'/>`.

```sh
ssh admin@<VM_IP_07> 'systemctl show -p RuntimeWatchdogUSec'
ssh admin@<VM_IP_07> 'ls -la /dev/watchdog'
```
**Esperado:** `RuntimeWatchdogUSec=20000000` (20s), `/dev/watchdog` existe.

**Teste de disparo** (⚠️ força um travamento real; não executado na última rodada — pendente):
```sh
ssh admin@<VM_IP_07> 'echo c | sudo tee /proc/sysrq-trigger'
# aguarde ~20-30s
virsh domstate test-07   # deve voltar a "running" sozinho (reset automático)
```

**Teste de rerun-mismatch:**
```sh
$VMCTL create --name=test-07   # sem --watchdog
```
**Esperado:** warning "VM 'test-07' already exists with a watchdog device, but this run did not request --watchdog." — watchdog continua ativo.

---

## 7. `on_crash=restart` (default ON) / `--no-crash-restart`

```sh
# test-07 (ou qualquer VM criada sem --no-crash-restart) já deve ter isso:
virsh dumpxml test-07 | grep on_crash
```
**Esperado:** `<on_crash>restart</on_crash>`.

**Teste de crash real (mata o processo QEMU, não a VM via virsh)** (⚠️ executado, mas terminou como achado não resolvido — ver abaixo, não tratar como "passou"):
```sh
QEMU_PID=$(ps aux | grep "[g]uest=test-07" | awk '{print $2}')
sudo kill -9 "$QEMU_PID"
sleep 5
virsh domstate test-07   # esperado: "running" de novo (libvirt reiniciou)
```

⚠️ **Achado de teste real (2026-07-20), não confirmado como bug**: rodando esse teste contra um host real (`libvirtd.service`, "legacy monolithic daemon"), a VM ficou `shut off` e nunca reiniciou sozinha, mesmo minutos depois — apesar de `<on_crash>restart</on_crash>` estar corretamente gravado na definição da VM. Hipótese: `<on_crash>` do libvirt rege o comportamento quando o **guest** reporta um crash (via `pvpanic` ou mecanismo similar) — não necessariamente quando o **processo QEMU do host** morre por `SIGKILL` externo, que pode ser mais parecido com "a energia caiu" do ponto de vista do libvirt do que com o evento que `on_crash` foi desenhado pra tratar. Ver `design.md`'s Open Questions para mais detalhes. Se for revisitar: teste via `virsh qemu-monitor-command` injetando um NMI/panic, ou um dispositivo `pvpanic` disparado de dentro do guest, em vez de `kill -9`.

**Teste do opt-out (VM nova):**
```sh
$VMCTL create --name=test-08 --no-crash-restart
QEMU_PID=$(ps aux | grep "[g]uest=test-08" | awk '{print $2}')
sudo kill -9 "$QEMU_PID"
sleep 5
virsh domstate test-08   # deve continuar "shut off" (sem reinício automático)
virsh start test-08       # precisa iniciar manualmente
```

---

## 8. `vmctl snapshot` / `vmctl backup`

Use `test-01` (ou qualquer VM já rodando).

### Snapshot (rollback rápido)
```sh
$VMCTL snapshot create --name=test-01
virsh snapshot-list test-01
```
**Esperado:** VM continua rodando; snapshot `self-hosting-snapshot` listado.

```sh
# tenta criar um segundo — deve falhar
$VMCTL snapshot create --name=test-01
```
**Esperado:** `ERROR: VM 'test-01' already has an active snapshot`.

```sh
# faz uma mudança dentro da VM (test-01 tem sudo com senha — usa -t)
ssh -t admin@<VM_IP> 'sudo touch /root/depois-do-snapshot.txt'

$VMCTL snapshot restore --name=test-01
# confirme com "y" no prompt
ssh admin@<VM_IP> 'ls /root/depois-do-snapshot.txt'   # NÃO deve existir mais
```

```sh
# repita snapshot + mudança, mas desta vez use 'vmctl snapshot delete' (mantém mudanças)
$VMCTL snapshot create --name=test-01
ssh -t admin@<VM_IP> 'sudo touch /root/mantido.txt'
$VMCTL snapshot delete --name=test-01
ssh admin@<VM_IP> 'ls /root/mantido.txt'   # DEVE existir
virsh domblklist test-01   # disco deve ser um único arquivo, sem overlay pendente
```

### Backup (cópia separada)
```sh
# VM rodando (live backup)
$VMCTL backup create --name=test-01
ls -la ~/vm-backups/test-01/
virsh domblklist test-01   # confirmar que não sobrou overlay depois do blockcommit

# VM parada (cópia direta)
virsh shutdown test-01
sleep 10
$VMCTL backup create --name=test-01
virsh start test-01
```

```sh
$VMCTL backup list --name=test-01
```
**Esperado:** lista os dois backups com timestamp.

**Teste de retenção:**
```sh
for i in 1 2 3; do $VMCTL backup create --name=test-01 --keep=2; done
ls ~/vm-backups/test-01/ | wc -l   # deve manter só os 2 mais recentes
```

**Teste de `vmctl backup restore`:**
```sh
BACKUP_FILE=$(ls -t ~/vm-backups/test-01/*.qcow2 | head -1)
$VMCTL backup restore --name=test-01 --file="$BACKUP_FILE"
# confirme com "y"
```
**Esperado:** VM volta ao estado do backup escolhido, reinicia se estava rodando antes.

⚠️ **Achado de teste real (2026-07-20)**: se a VM já passou por um `blockcommit --active --pivot` (via `vmctl snapshot delete` ou um `vmctl backup create` ao vivo) antes deste teste, o arquivo de disco (`<name>.qcow2`) pode acabar com dono `root:root` em vez do usuário atual — aparentemente o `libvirtd` assume a posse do arquivo ao finalizar o pivot. Como `qemu-img convert` aqui roda sem `sudo`, a escrita falha com `Permission denied`. Não é algo que o `vmctl` controla (`virsh blockcommit` é chamado normalmente) — é comportamento do libvirtd/QEMU. Ver `design.md`'s Open Questions. Se acontecer, resolva na mão antes de continuar:
```sh
sudo chown "$(whoami)" ~/vms/test-01/test-01.qcow2
```

### Confirma que `vmctl delete` nunca apaga backups
```sh
$VMCTL delete --name=test-01 --purge-all
ls ~/vm-backups/test-01/   # os arquivos de backup DEVEM continuar lá
```

---

## 9. `vmctl list` / `vmctl info`

Recrie ao menos duas VMs antes desta seção (ex: `test-01` e `test-05`), já que a seção 8 pode ter purgado `test-01`:
```sh
$VMCTL create --name=test-01
$VMCTL create --name=test-05 --ram=4096 --vcpus=4 --disk=30
```

```sh
$VMCTL list
```
**Esperado:** uma linha por VM definida (rodando ou parada), com colunas `NAME STATE RAM VCPUS DISK MODE IP`. `test-05` deve mostrar `4096`/`4`/`30G`. VMs paradas aparecem com `IP` como `-`, sem erro. A coluna `DISK` deve mostrar um tamanho real (ex. `20G`), não `-`, mesmo com a VM rodando (`qemu-img info` precisa de `-U`/`--force-share` pra não colidir com o lock do QEMU).

```sh
$VMCTL info --name=test-01
```
**Esperado:** a mesma linha de `test-01` isolada.

```sh
$VMCTL info --name=nao-existe-xyz
```
**Esperado:** `ERROR: no VM named 'nao-existe-xyz' found...`, sem imprimir tabela parcial.

**Teste de "sempre live, nunca cacheado"** (o motivo de existir `vmctl list` em vez de um arquivo de estado):
```sh
virsh destroy test-01
$VMCTL list   # deve refletir test-01 como parado IMEDIATAMENTE, sem passo de "invalidar cache"
virsh start test-01
$VMCTL list   # volta a "running" na mesma hora
```

**Teste de isolamento de falha por VM** (uma VM com introspecção quebrada não deve derrubar a listagem inteira):
```sh
# difícil de forçar deliberadamente; se alguma VM da frota estiver num estado
# esquisito (ex: domínio definido mas XML corrompido), confirme que `vmctl list`
# ainda lista as outras VMs normalmente e marca só aquela como "unknown".
```

---

## 10. Metadados consolidados (`meta.json`)

```sh
cat ~/vms/test-01/meta.json
```
**Esperado:** um JSON com `admin_sudo_policy`, `log_forwarding` e `guest_firewall_policy` (ex: `{"admin_sudo_policy": "nopasswd", "guest_firewall_policy": "enabled"}` — `log_forwarding` só aparece se `true`, por causa do `omitempty`).

**Teste de sobrevivência a `--vm-only`:**
```sh
$VMCTL delete --name=test-01 --vm-only
cat ~/vms/test-01/meta.json   # DEVE continuar existindo
$VMCTL create --name=test-01   # rerun rápido, reaproveitando a imagem base
cat ~/vms/test-01/meta.json   # deve refletir a config da VM recriada
```

**Teste de remoção em `--purge-all`:**
```sh
$VMCTL delete --name=test-01 --purge-all   # sem outras VMs
ls ~/vms/test-01/ 2>&1   # o diretório inteiro (e o meta.json dentro dele) deve ter sumido
```

**Teste de "metadado ausente = não configurado"** (simula uma VM criada antes deste recurso existir):
```sh
$VMCTL create --name=test-09
rm ~/vms/test-09/meta.json
$VMCTL create --name=test-09   # rerun contra a mesma VM, sem o arquivo
```
**Esperado:** não deve dar erro; trata a política de sudo/log-forwarding como não configuradas (equivalente ao comportamento antigo de "arquivo dotfile ausente").

---

## 11. Rename da CLI (`create`/`start`/`stop`/`reboot`/`delete`/`info`) + verbos `snapshot`/`backup`

Valida a change `vmctl-vps-style-cli`: os nomes antigos (`setup`/`cleanup`/`status`, `backup snapshot*`/`backup backup*`) não devem mais funcionar, e os novos verbos devem se comportar exatamente como os antigos se comportavam. Usa uma VM própria (`cli-test-01`) pra não interferir com as VMs de outras seções.

### 11.1 — 🟢 Nomes antigos não existem mais

```sh
$VMCTL setup; echo "exit: $?"
$VMCTL cleanup; echo "exit: $?"
$VMCTL status; echo "exit: $?"
$VMCTL backup snapshot; echo "exit: $?"
$VMCTL restore; echo "exit: $?"
```
**Esperado:** todos os cinco imprimem `ERROR: unknown subcommand: ...` (ou, no caso de `backup snapshot`, `unknown verb: snapshot` dentro do usage de `backup`) e saem com `exit: 1`.

### 11.2 — 🟢 Criar + ciclo de power (graceful)

```sh
$VMCTL create --name=cli-test-01; echo "exit: $?"
$VMCTL info --name=cli-test-01

$VMCTL stop --name=cli-test-01; echo "exit: $?"
sleep 5
$VMCTL info --name=cli-test-01   # deve mostrar "shut off"

$VMCTL start --name=cli-test-01; echo "exit: $?"
$VMCTL info --name=cli-test-01   # deve voltar a "running"

$VMCTL reboot --name=cli-test-01; echo "exit: $?"
```
**Esperado:** `stop` sem `--force` pede um shutdown ACPI (`virsh shutdown`) — pode levar alguns segundos pra `info` refletir `shut off`, daí o `sleep 5`. `start`/`reboot` idem, via `virsh start`/`virsh reboot`.

**Teste de idempotência:**
```sh
$VMCTL start --name=cli-test-01; echo "exit: $?"   # já está rodando
```
**Esperado:** mensagem `'cli-test-01' is already running.` e `exit: 0`, sem chamar `virsh start` de novo.

### 11.3 — 🟡 Caminho `--force`

```sh
$VMCTL stop --name=cli-test-01 --force; echo "exit: $?"
$VMCTL info --name=cli-test-01   # deve mostrar "shut off" IMEDIATAMENTE (virsh destroy é síncrono)

$VMCTL start --name=cli-test-01
$VMCTL reboot --name=cli-test-01 --force; echo "exit: $?"
```
**Esperado:** `--force` usa `virsh destroy`/`virsh reset` (hard), sem esperar ACPI — o `stop --force` deve refletir em `info` sem precisar de `sleep`.

### 11.4 — 🟢 Equivalência dos verbos `snapshot`/`backup`

```sh
$VMCTL snapshot create --name=cli-test-01; echo "exit: $?"
virsh snapshot-list cli-test-01

$VMCTL snapshot create --name=cli-test-01; echo "exit: $?"   # deve falhar, já existe um
$VMCTL snapshot delete --name=cli-test-01; echo "exit: $?"

$VMCTL backup create --name=cli-test-01; echo "exit: $?"
$VMCTL backup list --name=cli-test-01

BACKUP_FILE=$(ls -t ~/vm-backups/cli-test-01/*.qcow2 | head -1)
$VMCTL backup restore --name=cli-test-01 --file="$BACKUP_FILE"; echo "exit: $?"
# confirme com "y" no prompt
```
**Esperado:** comportamento idêntico ao que `vmctl backup snapshot`/`snapshot-delete`/`backup`/`backup-list`/`backup-restore` já faziam (seção 8) — só o nome do verbo mudou.

### 11.5 — 🟢 Escopo de `delete --vm-only`

```sh
$VMCTL delete --name=cli-test-01 --vm-only; echo "exit: $?"
virsh list --all   # cli-test-01 não deve mais existir
ls ~/vm-backups/cli-test-01/   # os backups DEVEM continuar lá (delete nunca apaga backups)
```
**Esperado:** mesmo escopo que `vmctl cleanup --vm-only` já tinha depois da change `vmctl-host-doctor` (só a VM, nada de host).

**Reporte:** cole a saída de cada bloco (11.1 a 11.5).

---

## Limpeza final

```sh
for vm in test-01 test-02 test-03 test-04 test-05 test-06 test-07 test-08 test-09 cli-test-01; do
  virsh dominfo "$vm" >/dev/null 2>&1 && $VMCTL delete --name="$vm" --vm-only
done
# some backups/logs de teste ficam preservados de propósito — apague manualmente se quiser:
# rm -rf ~/vm-backups/test-* ~/vm-backups/cli-test-* /var/log/self-hosting-vms/test-*
```

**Teste de `vmctl doctor --unfix`** (opcional — só se quiser desfazer o `--fix` da seção 0 por completo; deixa o host sem KVM/libvirt):
```sh
virsh list --all   # confirme que está vazio antes de continuar

$VMCTL doctor --unfix
$VMCTL doctor   # tudo deve voltar a aparecer [MISSING]
```
**Esperado:** com pelo menos uma VM ainda definida, `--unfix` deve se recusar a rodar (erro listando a(s) VM(s) e apontando pra removê-las primeiro com `vmctl delete --vm-only`). Só com `virsh list --all` vazio ele deve prosseguir: remove a rede `default`, purga os pacotes, remove o usuário dos grupos `libvirt`/`kvm`, e revoga a ACL do `libvirt-qemu` em `$HOME`.
