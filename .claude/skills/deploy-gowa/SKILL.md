---
name: deploy-gowa
description: Compila o GOWA no Undying, publica no LXC 139 e verifica. Use apenas para uma invocação explícita de /deploy-gowa.
disable-model-invocation: true
---

# Deploy GOWA

Publica o binário do fork no LXC 139. **Só correr após confirmação explícita do
utilizador** — reinicia um gateway de WhatsApp em produção, com ~20 s de downtime.

## Topologia

| | |
|---|---|
| Build | Undying (esta máquina). Arch, glibc 2.44 |
| Alvo | LXC 139 `gowa`, Debian 13 (trixie), glibc 2.41 |
| Acesso | **Não há host SSH direto.** Via `ssh vibranium` + `pct exec 139` |
| Binário | `/opt/gowa/bin/whatsapp`, dono `gowa:gowa`, modo 0755 |
| Unit | `gowa.service`, `WorkingDirectory=/opt/gowa`, `EnvironmentFile=/etc/gowa/gowa.env` |
| Público | `https://gowa.atlantida-code.com.br` via nginx no vibranium |

Não há Go no container — o binário é sempre construído aqui e empurrado.

## Passos

### 1. Pré-verificação local

```bash
cd src && go build ./... && go vet ./... && go test ./...
```

Confirmar que o delta do fork está presente, senão o deploy reintroduz o bug do
Chatwoot:

```bash
grep -q escapeChatwootPathSegment src/infrastructure/chatwoot/client.go
```

### 2. Build e verificação de GLIBC

```bash
cd src && go build -o /tmp/gowa-build/whatsapp .
readelf -V /tmp/gowa-build/whatsapp | grep -oE 'GLIBC_[0-9.]+' | sort -uV | tail -3
```

**O símbolo mais alto tem de ser ≤ `GLIBC_2.41`.** A máquina de build é mais nova
que o alvo; se o Arch subir a glibc, um build com CGO pode exigir símbolos que o
Debian 13 não tem e o serviço morre no arranque. Hoje o build pede no máximo
`GLIBC_2.34`. Se exceder, parar e reportar — a alternativa é
`CGO_ENABLED=0 go build -tags purego`, mas isso troca o driver SQLite e é uma
mudança separada, que não se faz no mesmo deploy.

### 3. Transferir, conferindo o sha256 nas três pontas

```bash
sha256sum /tmp/gowa-build/whatsapp
scp /tmp/gowa-build/whatsapp vibranium:/tmp/whatsapp-novo
ssh vibranium 'sha256sum /tmp/whatsapp-novo'
ssh vibranium 'pct push 139 /tmp/whatsapp-novo /opt/gowa/bin/whatsapp.new --user 0 --group 0 --perms 0755'
ssh vibranium 'pct exec 139 -- sha256sum /opt/gowa/bin/whatsapp.new'
```

### 4. Migrations — verificar antes de parar o serviço

```bash
cd src && awk '/^func \(r \*SQLiteRepository\) getMigrations/,/^}$/' \
  infrastructure/chatstorage/sqlite_repository.go | grep -c '`'
```

Dividir por 2 = número de migrations. Comparar com a versão em produção. **Se
aumentou, o rollback deixa de ser só trocar o binário** — dizê-lo ao utilizador
antes de continuar, porque o schema avança e o binário antigo pode não o
entender.

### 5. Parar, salvaguardar, trocar, arrancar

Parar **antes** de copiar o `chatstorage.db`: o WAL faz checkpoint no shutdown
limpo e a cópia fica consistente. Copiar com o serviço a correr apanha um WAL
solto.

```bash
ssh vibranium 'pct exec 139 -- bash -lc "set -e
systemctl stop gowa; sleep 2
mkdir -p /opt/gowa/backup-\$(date +%Y%m%d)
cp -a /opt/gowa/storages/*.db* /opt/gowa/backup-\$(date +%Y%m%d)/
cp -a /opt/gowa/bin/whatsapp /opt/gowa/bin/whatsapp.bak-<versao-anterior>
mv /opt/gowa/bin/whatsapp.new /opt/gowa/bin/whatsapp
chown gowa:gowa /opt/gowa/bin/whatsapp && chmod 0755 /opt/gowa/bin/whatsapp
systemctl start gowa"'
```

Nomear o `.bak` pela versão que sai (ex.: `whatsapp.bak-v9.1.0-fork-escape`), não
por data — é assim que se distinguem as cópias existentes.

**A sessão do whatsmeow está em Postgres** (`DB_URI` → 10.10.10.116), não neste
backup. O `chatstorage.db` local guarda chats/mensagens e o estado do Chatwoot.
Se o update trouxer um bump de whatsmeow, avisar que um rollback do binário não
reverte migrations que o whatsmeow tenha corrido na sua própria store.

### 6. Verificar

```bash
ssh vibranium 'pct exec 139 -- bash -lc "
systemctl is-active gowa
curl -s -o /dev/null -w \"health=%{http_code}\n\" http://127.0.0.1:3000/health
journalctl -u gowa --since \"-3min\" --no-pager | grep -ciE \"level=error|panic|fatal\"
journalctl -u gowa --since \"-3min\" --no-pager | grep -iE \"auto-connected|presence pulse\""'
```

Verde é: `active`, `health=200`, **0** erros, e uma linha `auto-connected device`
— esta última confirma que a sessão do WhatsApp sobreviveu. Sem ela, o device não
religou e é preciso investigar antes de dar o deploy por bom.

Confirmar a versão no binário (não há `strings` no container):

```bash
ssh vibranium 'pct exec 139 -- grep -aoE "v9\.[0-9]+\.[0-9]+" /opt/gowa/bin/whatsapp | sort -u | head -3'
```

### 7. Limpar e reportar

Apagar `/tmp/whatsapp-novo` no vibranium. Reportar a versão publicada, o downtime
observado (comparar `Deactivated successfully` com `Active: since` no journal) e
imprimir o comando de rollback:

```bash
ssh vibranium 'pct exec 139 -- bash -c "cp /opt/gowa/bin/whatsapp.bak-<versao> /opt/gowa/bin/whatsapp && systemctl restart gowa"'
```

## Se falhar o arranque

`systemctl start` a devolver sucesso não garante nada — o processo pode morrer a
seguir. Verificar sempre `is-active` **depois** de esperar alguns segundos.

Causas conhecidas de `Fatalln` no arranque, todas em `cmd/rest.go`:

- `APP_BASIC_AUTH` malformado (entrada sem exatamente um `:`)
- `MCP_OAUTH_ENABLED=true` com `APP_BASIC_AUTH` vazio ou issuer que não seja HTTPS absoluto
- chat storage inacessível

O journal dá a linha exata. Rollback imediato se não for óbvio.
