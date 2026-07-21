# Achados e bugs

Registro consolidado de comportamentos inesperados encontrados em testes contra hosts reais (KVM/libvirt de verdade, não simuláveis em CI/sandbox). Bugs já corrigidos com uma causa raiz simples e local ficam documentados como comentário no próprio código, perto do fix — este arquivo é para achados que precisam de atenção continuada, têm causa raiz não totalmente entendida, ou têm valor histórico de mais alto nível.

---

## [RESOLVIDO] `vmctl doctor --fix` não conseguia redefinir a rede `default` depois do `--unfix`

**Quando:** 2026-07-21, durante a validação da change `vmctl-host-doctor`.

`vmctl doctor --unfix` roda `virsh net-undefine default`, que remove a definição da rede por completo (não só desativa). `ensureNATNetworkReady` (dentro do `Fix`) só sabia ativar/configurar autostart de uma rede **já definida** — nunca soube redefini-la do zero. O ciclo completo `--fix` → `--unfix` → `--fix` nunca tinha sido exercitado antes dessa rodada de teste.

**Fix:** `Fix` agora roda `virsh net-define /usr/share/libvirt/networks/default.xml` quando a rede não existe, antes de tentar ativá-la/autostart. Ver `vmctl/internal/hostready/fix.go`.

---

## [ABERTO] `on_crash=restart` não recupera a VM de um `kill -9` no processo QEMU do host

**Quando:** 2026-07-20, testado contra um host real (`libvirtd.service`, "legacy monolithic daemon").

Uma VM criada com `on_crash=restart` (default, config confirmada correta e idêntica ao que o bash geraria) ficou `shut off` indefinidamente depois que seu processo QEMU foi morto com `SIGKILL` — nenhuma tentativa de restart foi registrada em lugar nenhum (`journalctl -u libvirtd`, `systemd-machined`, etc.).

**Hipótese:** `on_crash` do libvirt pode reger o comportamento apenas quando o **guest** reporta um crash (via `pvpanic` ou mecanismo similar) — não necessariamente quando o **processo QEMU do host** morre por `SIGKILL` externo, que do ponto de vista do libvirt pode ser mais parecido com "a energia caiu" do que com o evento que `on_crash` foi desenhado pra tratar.

**Não é uma regressão vmctl-vs-bash** — o XML do domínio gerado é comprovadamente idêntico nos dois casos.

**Se for revisitar:** testar via `virsh qemu-monitor-command` injetando um NMI/panic, ou um dispositivo `pvpanic` disparado de dentro do guest, em vez de `kill -9` no processo do host.

---

## [ABERTO, com workaround] `blockcommit --active --pivot` deixa o disco com dono `root:root`

**Quando:** 2026-07-20, testado contra um host real.

Depois que uma VM passa por um `virsh blockcommit --active --pivot` (usado por `snapshot delete` e pelo caminho de `backup create` ao vivo), o arquivo de disco (`<name>.qcow2`) pode acabar com dono `root:root` em vez do usuário atual — aparentemente o `libvirtd` (rodando como root) assume a posse do arquivo ao finalizar o pivot. Como `qemu-img convert` roda sem `sudo`, uma restauração de backup subsequente falha com `Permission denied`.

**Não é algo que o `vmctl` controla** — `virsh blockcommit` é chamado da mesma forma que o bash chamava; é comportamento do libvirtd/QEMU.

**Deliberadamente não "corrigido"** fazendo o `vmctl` rechown o disco depois de todo blockcommit: o mecanismo exato ainda não é entendido, e blockcommit é chamado de três lugares (`cmdSnapshotDelete`, o caminho ao vivo de `cmdBackup`, e indiretamente durante a retenção) — um chown defensivo adicionado sem entender por que o drift acontece arrisca mascarar um problema real em vez de corrigi-lo.

**Workaround:** antes de restaurar, rode `sudo chown "$(whoami)" ~/vms/<name>/<name>.qcow2`.

---

## [ABERTO, mitigado] `virsh metadata` nunca foi testado sob carga real

**Quando:** durante a migração para o `vmctl` (Go), 2026-07-21.

Não foi possível testar como `virsh metadata` se comporta sob um spike real (tamanho do payload, escaping, fidelidade de round-trip) para os três fatos que só o guest sabe — o ambiente de implementação não tinha uma conexão `libvirtd` funcional pra isso.

**Mitigação:** o `vmctl` usa um fallback em arquivo JSON (`meta.json` dentro do working dir da VM) em vez de depender de `virsh metadata` como fonte única. Revisitar se `virsh metadata` nativo virar necessário no futuro (ex: pra sobreviver à remoção do working dir).
