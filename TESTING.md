# Manual test roteiro

Passo a passo para validar, num host real com KVM, as 8 mudanças implementadas nesta sessão. Cada seção é independente, mas assume que os pré-requisitos abaixo já rodaram uma vez.

Nenhum destes testes foi (ou pôde ser) executado em CI/sandbox — todos exigem uma VM real. Rode-os na ordem sugerida; várias seções reaproveitam a mesma VM (`test-01`) para economizar tempo de boot.

## Pré-requisitos (uma vez)

```sh
chmod +x scripts/debian-vm-setup.sh scripts/debian-vm-cleanup.sh scripts/debian-vm-backup.sh
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
./scripts/debian-vm-setup.sh --name=test-01 --admin-password
```

**Esperado:**
- Output mostra a senha gerada uma vez, e o caminho `~/vms/test-01/admin-password` (arquivo com `chmod 600`).
- Nota final menciona `virsh console test-01` como escape hatch.

```sh
# Confirma permissões do arquivo de senha
stat -c '%a' ~/vms/test-01/admin-password   # deve ser 600
cat ~/vms/test-01/.admin-sudo-policy         # deve conter "password-required"

# Confirma que SSH continua só-chave (não deve pedir senha)
ssh -o PasswordAuthentication=no admin@$(virsh domifaddr test-01 | awk '/ipv4/{print $4}' | cut -d/ -f1)

# Dentro da VM, confirma que sudo agora pede senha
ssh admin@<VM_IP> 'sudo -n true' # deve FALHAR (sudo exige senha, -n = non-interactive)
ssh -t admin@<VM_IP> 'sudo whoami'  # deve pedir a senha do arquivo admin-password
```

**Teste de rerun-mismatch:**
```sh
./scripts/debian-vm-setup.sh --name=test-01   # sem --admin-password, mesma VM
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
./scripts/debian-vm-setup.sh --name=test-02 --no-auto-updates
ssh admin@<VM_IP_02> 'dpkg -l unattended-upgrades'   # deve dizer "no packages found"
```

---

## 3. Guest firewall (`--allow-port`, `--no-guest-firewall`) + fail2ban

```sh
# test-01 já tem o firewall padrão (default-deny). Confirme:
ssh admin@<VM_IP> 'sudo ufw status'
ssh admin@<VM_IP> 'sudo fail2ban-client status sshd'
```
**Esperado:** `Status: active`, `22/tcp ALLOW`, política default `deny (incoming), allow (outgoing)`. Jail `sshd` ativo.

**Teste de `--allow-port` + `--forward` (VM nova):**
```sh
./scripts/debian-vm-setup.sh --name=test-03 --allow-port=8080 --forward=9000:80
ssh admin@<VM_IP_03> 'sudo ufw status numbered'
```
**Esperado:** portas 22, 80 (derivada do `--forward`) e 8080 (`--allow-port`) todas `ALLOW`.

**Teste do warning de `--forward` tardio** (o achado do modo exploração):
```sh
./scripts/debian-vm-setup.sh --name=test-03 --forward=9001:81
```
**Esperado:** a regra DNAT/FORWARD é aplicada no host normalmente, MAS aparece:
```
WARNING: this VM's guest firewall (ufw) was enabled at creation, but its allow
         list can't be updated by rerunning this script...
           ssh admin@<VM_IP_03> sudo ufw allow 81/tcp
```
Confirme que a porta 81 NÃO está liberada dentro do guest até você rodar o comando sugerido manualmente.

**Teste do opt-out:**
```sh
./scripts/debian-vm-setup.sh --name=test-04 --no-guest-firewall --allow-port=9999
ssh admin@<VM_IP_04> 'which ufw'   # não deve existir
ssh admin@<VM_IP_04> 'sudo fail2ban-client status sshd'   # deve continuar ativo (não é afetado pela flag)
```

---

## 4. `--harden-host-firewall` (firewall do HOST, cuidado)

⚠️ Isso reconfigura o firewall da sua máquina física. Rode num host de teste, ou tenha acesso físico/console de emergência caso algo saia errado com SSH.

```sh
./scripts/debian-vm-setup.sh --name=test-01 --harden-host-firewall
sudo ufw status verbose
```
**Esperado:** `ufw` ativo no HOST, regra `22/tcp ALLOW ... # self-hosting: host SSH baseline`, `DEFAULT_FORWARD_POLICY="ACCEPT"` em `/etc/default/ufw`.

**Confirma que o NAT/forward continuam funcionando:**
```sh
# se test-01 tem --forward configurado de testes anteriores, confirme que ainda funciona:
curl -m3 http://localhost:<HOST_PORT>   # ou o que estiver exposto
ssh admin@$(virsh domifaddr test-01 | awk '/ipv4/{print $4}' | cut -d/ -f1)   # NAT interno continua ok
```

**Teste de idempotência:**
```sh
./scripts/debian-vm-setup.sh --name=test-01 --harden-host-firewall   # roda de novo
sudo ufw status numbered | grep -c "host SSH baseline"   # deve ser exatamente 1, não 2
```

**Teste de remoção (cleanup.sh):**
```sh
./scripts/debian-vm-cleanup.sh --name=test-01 --vm-only
sudo ufw status | grep "host SSH baseline"   # deve AINDA existir (--vm-only preserva)

./scripts/debian-vm-cleanup.sh --name=test-01 --purge-all   # com nenhuma outra VM ativa
sudo ufw status | grep "host SSH baseline"   # não deve mais existir
grep DEFAULT_FORWARD_POLICY /etc/default/ufw   # deve voltar a "DROP"
which ufw   # ufw continua instalado
```

---

## 5. `--monitor` (uptime, logging, alerting)

```sh
./scripts/debian-vm-setup.sh --name=test-05 --monitor
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

**Teste do motd:**
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

**Teste com `--bridge` (logging deve ficar indisponível, uptime não):**
```sh
./scripts/debian-vm-setup.sh --name=test-06 --bridge=<sua-interface> --monitor
```
**Esperado:** nota explícita "Centralized logging is NOT available in bridged mode". `systemctl list-timers` ainda mostra o timer de test-06.

**Teste de cleanup:**
```sh
./scripts/debian-vm-cleanup.sh --name=test-05 --vm-only
systemctl is-enabled self-hosting-vm-uptime@test-05.timer   # deve estar "disabled"/inexistente
ls /var/log/self-hosting-vms/test-05/   # logs devem continuar lá

./scripts/debian-vm-cleanup.sh --name=test-05 --purge-all   # sem outras VMs
# deve perguntar "Delete accumulated VM logs...?" mesmo em modo não-interativo de outras etapas
```

---

## 6. `--watchdog`

```sh
./scripts/debian-vm-setup.sh --name=test-07 --watchdog
virsh dumpxml test-07 | grep -A1 '<watchdog'
```
**Esperado:** `<watchdog model='i6300esb' action='reset'/>`.

```sh
ssh admin@<VM_IP_07> 'systemctl show -p RuntimeWatchdogUSec'
ssh admin@<VM_IP_07> 'ls -la /dev/watchdog'
```
**Esperado:** `RuntimeWatchdogUSec=20000000` (20s), `/dev/watchdog` existe.

**Teste de disparo (⚠️ força um travamento real):**
```sh
ssh admin@<VM_IP_07> 'echo c | sudo tee /proc/sysrq-trigger'
# aguarde ~20-30s
virsh domstate test-07   # deve voltar a "running" sozinho (reset automático)
```

**Teste de rerun-mismatch:**
```sh
./scripts/debian-vm-setup.sh --name=test-07   # sem --watchdog
```
**Esperado:** warning "VM 'test-07' already exists with a watchdog device, but this run did not request --watchdog." — watchdog continua ativo.

---

## 7. `on_crash=restart` (default ON) / `--no-crash-restart`

```sh
# test-07 (ou qualquer VM criada sem --no-crash-restart) já deve ter isso:
virsh dumpxml test-07 | grep on_crash
```
**Esperado:** `<on_crash>restart</on_crash>`.

**Teste de crash real (mata o processo QEMU, não a VM via virsh):**
```sh
QEMU_PID=$(ps aux | grep "[g]uest=test-07" | awk '{print $2}')
sudo kill -9 "$QEMU_PID"
sleep 5
virsh domstate test-07   # deve estar "running" de novo (libvirt reiniciou)
```

**Teste do opt-out (VM nova):**
```sh
./scripts/debian-vm-setup.sh --name=test-08 --no-crash-restart
QEMU_PID=$(ps aux | grep "[g]uest=test-08" | awk '{print $2}')
sudo kill -9 "$QEMU_PID"
sleep 5
virsh domstate test-08   # deve continuar "shut off" (sem reinício automático)
virsh start test-08       # precisa iniciar manualmente
```

---

## 8. `debian-vm-backup.sh` (snapshot + backup)

Use `test-01` (ou qualquer VM já rodando).

### Snapshot (rollback rápido)
```sh
./scripts/debian-vm-backup.sh snapshot --name=test-01
virsh snapshot-list test-01
```
**Esperado:** VM continua rodando; snapshot `self-hosting-snapshot` listado.

```sh
# tenta criar um segundo — deve falhar
./scripts/debian-vm-backup.sh snapshot --name=test-01
```
**Esperado:** `ERROR: VM 'test-01' already has an active snapshot`.

```sh
# faz uma mudança dentro da VM
ssh admin@<VM_IP> 'sudo touch /root/depois-do-snapshot.txt'

./scripts/debian-vm-backup.sh snapshot-restore --name=test-01
# confirme com "y" no prompt
ssh admin@<VM_IP> 'ls /root/depois-do-snapshot.txt'   # NÃO deve existir mais
```

```sh
# repita snapshot + mudança, mas desta vez use snapshot-delete (mantém mudanças)
./scripts/debian-vm-backup.sh snapshot --name=test-01
ssh admin@<VM_IP> 'sudo touch /root/mantido.txt'
./scripts/debian-vm-backup.sh snapshot-delete --name=test-01
ssh admin@<VM_IP> 'ls /root/mantido.txt'   # DEVE existir
virsh domblklist test-01   # disco deve ser um único arquivo, sem overlay pendente
```

### Backup (cópia separada)
```sh
# VM rodando (live backup)
./scripts/debian-vm-backup.sh backup --name=test-01
ls -la ~/vm-backups/test-01/
virsh domblklist test-01   # confirmar que não sobrou overlay depois do blockcommit

# VM parada (cópia direta)
virsh shutdown test-01
sleep 10
./scripts/debian-vm-backup.sh backup --name=test-01
virsh start test-01
```

```sh
./scripts/debian-vm-backup.sh backup-list --name=test-01
```
**Esperado:** lista os dois backups com timestamp.

**Teste de retenção:**
```sh
for i in 1 2 3; do ./scripts/debian-vm-backup.sh backup --name=test-01 --keep=2; done
ls ~/vm-backups/test-01/ | wc -l   # deve manter só os 2 mais recentes
```

**Teste de `backup-restore`:**
```sh
BACKUP_FILE=$(ls -t ~/vm-backups/test-01/*.qcow2 | head -1)
./scripts/debian-vm-backup.sh backup-restore --name=test-01 --file="$BACKUP_FILE"
# confirme com "y"
```
**Esperado:** VM volta ao estado do backup escolhido, reinicia se estava rodando antes.

### Confirma que cleanup.sh nunca apaga backups
```sh
./scripts/debian-vm-cleanup.sh --name=test-01 --purge-all
ls ~/vm-backups/test-01/   # os arquivos de backup DEVEM continuar lá
```

---

## Limpeza final

```sh
for vm in test-01 test-02 test-03 test-04 test-05 test-06 test-07 test-08; do
  virsh dominfo "$vm" >/dev/null 2>&1 && ./scripts/debian-vm-cleanup.sh --name="$vm" --vm-only
done
# some backups/logs de teste ficam preservados de propósito — apague manualmente se quiser:
# rm -rf ~/vm-backups/test-* /var/log/self-hosting-vms/test-*
```
