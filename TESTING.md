# Manual test roteiro

Passo a passo para validar, num host real com KVM, as mudanças implementadas no `vmctl` (o binário Go que substitui os antigos `debian-vm-setup.sh`, `debian-vm-cleanup.sh` e `debian-vm-backup.sh`), incluindo `vmctl list`/`status` e os metadados consolidados (`meta.json`).

Nenhum destes testes foi (ou pôde ser) executado em CI/sandbox — todos exigem uma VM real. Rode-os na ordem sugerida; várias seções reaproveitam a mesma VM (`test-01`) para economizar tempo de boot.

**Status da última rodada completa (2026-07-20/21):** a maior parte do roteiro foi executada contra um host real, o que revelou e corrigiu 7 bugs reais (ver `design.md` e o histórico de commits). Os seguintes pontos ficaram **sem cobertura** nessa rodada e continuam pendentes de validação:
- **`--bridge` (modo bridged)**, seções 5 e 9 — pulado por falta de interface cabeada disponível no host de teste.
- Disparo real do watchdog (`echo c > /proc/sysrq-trigger`, seção 6) — não executado.
- Teste do motd via logout/login na sessão do host (seção 5) — não executado.
- Seção 7 (`on_crash=restart` via `kill -9`) — executado, mas terminou como achado **não resolvido** (a VM não reiniciou sozinha); ver a nota ⚠️ na seção e `design.md`'s Open Questions. Não tratar como "passou".

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

## 1. `--admin-password` (sudo com senha)

```sh
# Cria a VM com senha de sudo
$VMCTL setup --name=test-01 --admin-password
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
$VMCTL setup --name=test-01   # sem --admin-password, mesma VM
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
$VMCTL setup --name=test-02 --no-auto-updates
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
$VMCTL setup --name=test-03 --allow-port=8080 --forward=9000:80
ssh admin@<VM_IP_03> 'sudo ufw status numbered'
```
**Esperado:** portas 22, 80 (derivada do `--forward`) e 8080 (`--allow-port`) todas `ALLOW`.

**Teste do warning de `--forward` tardio:**
```sh
$VMCTL setup --name=test-03 --forward=9001:81
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
$VMCTL setup --name=test-04 --no-guest-firewall --allow-port=9999
ssh admin@<VM_IP_04> 'which ufw'   # não deve existir
ssh admin@<VM_IP_04> 'sudo fail2ban-client status sshd'   # deve continuar ativo (não é afetado pela flag)
```

---

## 4. `--harden-host-firewall` (firewall do HOST, cuidado)

⚠️ Isso reconfigura o firewall da sua máquina física. Rode num host de teste, ou tenha acesso físico/console de emergência caso algo saia errado com SSH.

```sh
$VMCTL setup --name=test-01 --harden-host-firewall
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
$VMCTL setup --name=test-01 --harden-host-firewall   # roda de novo
sudo ufw status numbered | grep -c "host SSH baseline"
```
**Esperado:** com IPv6 habilitado no ufw (padrão), `ufw allow ... comment "..."` cria uma regra v4 **e** uma v6 — então o valor esperado é **2**, estável entre reruns (o que importa é não crescer pra 4 na segunda vez, não ser exatamente 1).

**Teste de remoção (`vmctl cleanup`):**
```sh
$VMCTL cleanup --name=test-01 --vm-only
sudo ufw status | grep "host SSH baseline"   # deve AINDA existir (--vm-only preserva)

$VMCTL cleanup --name=test-01 --purge-all   # com nenhuma outra VM ativa
sudo ufw status | grep "host SSH baseline"   # não deve mais existir
grep DEFAULT_FORWARD_POLICY /etc/default/ufw   # deve voltar a "DROP"
which ufw   # ufw continua instalado
```

---

## 5. `--monitor` (uptime, logging, alerting)

```sh
$VMCTL setup --name=test-05 --monitor
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
$VMCTL setup --name=test-06 --bridge=<sua-interface> --monitor
```
**Esperado:** nota explícita "Centralized logging is NOT available in bridged mode". `systemctl list-timers` ainda mostra o timer de test-06.

**Teste de cleanup:**
```sh
$VMCTL cleanup --name=test-05 --vm-only
systemctl is-enabled self-hosting-vm-uptime@test-05.timer   # deve estar "disabled"/inexistente
ls /var/log/self-hosting-vms/test-05/   # logs devem continuar lá

$VMCTL cleanup --name=test-05 --purge-all   # sem outras VMs
# deve perguntar "Delete accumulated VM logs...?" mesmo em modo não-interativo de outras etapas
```

---

## 6. `--watchdog`

```sh
$VMCTL setup --name=test-07 --watchdog
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
$VMCTL setup --name=test-07   # sem --watchdog
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
$VMCTL setup --name=test-08 --no-crash-restart
QEMU_PID=$(ps aux | grep "[g]uest=test-08" | awk '{print $2}')
sudo kill -9 "$QEMU_PID"
sleep 5
virsh domstate test-08   # deve continuar "shut off" (sem reinício automático)
virsh start test-08       # precisa iniciar manualmente
```

---

## 8. `vmctl backup` (snapshot + backup)

Use `test-01` (ou qualquer VM já rodando).

### Snapshot (rollback rápido)
```sh
$VMCTL backup snapshot --name=test-01
virsh snapshot-list test-01
```
**Esperado:** VM continua rodando; snapshot `self-hosting-snapshot` listado.

```sh
# tenta criar um segundo — deve falhar
$VMCTL backup snapshot --name=test-01
```
**Esperado:** `ERROR: VM 'test-01' already has an active snapshot`.

```sh
# faz uma mudança dentro da VM (test-01 tem sudo com senha — usa -t)
ssh -t admin@<VM_IP> 'sudo touch /root/depois-do-snapshot.txt'

$VMCTL backup snapshot-restore --name=test-01
# confirme com "y" no prompt
ssh admin@<VM_IP> 'ls /root/depois-do-snapshot.txt'   # NÃO deve existir mais
```

```sh
# repita snapshot + mudança, mas desta vez use snapshot-delete (mantém mudanças)
$VMCTL backup snapshot --name=test-01
ssh -t admin@<VM_IP> 'sudo touch /root/mantido.txt'
$VMCTL backup snapshot-delete --name=test-01
ssh admin@<VM_IP> 'ls /root/mantido.txt'   # DEVE existir
virsh domblklist test-01   # disco deve ser um único arquivo, sem overlay pendente
```

### Backup (cópia separada)
```sh
# VM rodando (live backup)
$VMCTL backup backup --name=test-01
ls -la ~/vm-backups/test-01/
virsh domblklist test-01   # confirmar que não sobrou overlay depois do blockcommit

# VM parada (cópia direta)
virsh shutdown test-01
sleep 10
$VMCTL backup backup --name=test-01
virsh start test-01
```

```sh
$VMCTL backup backup-list --name=test-01
```
**Esperado:** lista os dois backups com timestamp.

**Teste de retenção:**
```sh
for i in 1 2 3; do $VMCTL backup backup --name=test-01 --keep=2; done
ls ~/vm-backups/test-01/ | wc -l   # deve manter só os 2 mais recentes
```

**Teste de `backup-restore`:**
```sh
BACKUP_FILE=$(ls -t ~/vm-backups/test-01/*.qcow2 | head -1)
$VMCTL backup backup-restore --name=test-01 --file="$BACKUP_FILE"
# confirme com "y"
```
**Esperado:** VM volta ao estado do backup escolhido, reinicia se estava rodando antes.

⚠️ **Achado de teste real (2026-07-20)**: se a VM já passou por um `blockcommit --active --pivot` (via `snapshot-delete` ou um `backup` ao vivo) antes deste teste, o arquivo de disco (`<name>.qcow2`) pode acabar com dono `root:root` em vez do usuário atual — aparentemente o `libvirtd` assume a posse do arquivo ao finalizar o pivot. Como `qemu-img convert` aqui roda sem `sudo`, a escrita falha com `Permission denied`. Não é algo que o `vmctl` controla (`virsh blockcommit` é chamado normalmente) — é comportamento do libvirtd/QEMU. Ver `design.md`'s Open Questions. Se acontecer, resolva na mão antes de continuar:
```sh
sudo chown "$(whoami)" ~/vms/test-01/test-01.qcow2
```

### Confirma que `vmctl cleanup` nunca apaga backups
```sh
$VMCTL cleanup --name=test-01 --purge-all
ls ~/vm-backups/test-01/   # os arquivos de backup DEVEM continuar lá
```

---

## 9. `vmctl list` / `vmctl status`

Recrie ao menos duas VMs antes desta seção (ex: `test-01` e `test-05`), já que a seção 8 pode ter purgado `test-01`:
```sh
$VMCTL setup --name=test-01
$VMCTL setup --name=test-05 --ram=4096 --vcpus=4 --disk=30
```

```sh
$VMCTL list
```
**Esperado:** uma linha por VM definida (rodando ou parada), com colunas `NAME STATE RAM VCPUS DISK MODE IP`. `test-05` deve mostrar `4096`/`4`/`30G`. VMs paradas aparecem com `IP` como `-`, sem erro. A coluna `DISK` deve mostrar um tamanho real (ex. `20G`), não `-`, mesmo com a VM rodando (`qemu-img info` precisa de `-U`/`--force-share` pra não colidir com o lock do QEMU).

```sh
$VMCTL status --name=test-01
```
**Esperado:** a mesma linha de `test-01` isolada.

```sh
$VMCTL status --name=nao-existe-xyz
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
$VMCTL cleanup --name=test-01 --vm-only
cat ~/vms/test-01/meta.json   # DEVE continuar existindo
$VMCTL setup --name=test-01   # rerun rápido, reaproveitando a imagem base
cat ~/vms/test-01/meta.json   # deve refletir a config da VM recriada
```

**Teste de remoção em `--purge-all`:**
```sh
$VMCTL cleanup --name=test-01 --purge-all   # sem outras VMs
ls ~/vms/test-01/ 2>&1   # o diretório inteiro (e o meta.json dentro dele) deve ter sumido
```

**Teste de "metadado ausente = não configurado"** (simula uma VM criada antes deste recurso existir):
```sh
$VMCTL setup --name=test-09
rm ~/vms/test-09/meta.json
$VMCTL setup --name=test-09   # rerun contra a mesma VM, sem o arquivo
```
**Esperado:** não deve dar erro; trata a política de sudo/log-forwarding como não configuradas (equivalente ao comportamento antigo de "arquivo dotfile ausente").

---

## Limpeza final

```sh
for vm in test-01 test-02 test-03 test-04 test-05 test-06 test-07 test-08 test-09; do
  virsh dominfo "$vm" >/dev/null 2>&1 && $VMCTL cleanup --name="$vm" --vm-only
done
# some backups/logs de teste ficam preservados de propósito — apague manualmente se quiser:
# rm -rf ~/vm-backups/test-* /var/log/self-hosting-vms/test-*
```
