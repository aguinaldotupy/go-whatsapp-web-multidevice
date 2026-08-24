---
name: sync-upstream
description: Traz commits novos do aldinokemal/go-whatsapp-web-multidevice para o fork, confirmando que o delta local sobrevive ao merge. Use apenas para uma invocação explícita de /sync-upstream.
disable-model-invocation: true
---

# Sync upstream

Este repo é o fork privado `GOWA-ATCode/go-whatsapp-web-multidevice`. O `upstream`
é `aldinokemal/go-whatsapp-web-multidevice` (MIT, público).

## O delta local que não pode perder-se

`escapeChatwootPathSegment` em `src/infrastructure/chatwoot/client.go`, mais
`src/infrastructure/chatwoot/escape_path_test.go`.

**O upstream continua com o bug** — usa `url.PathEscape` cru, e o router do Rails
do Chatwoot não resolve `@` nem `.` num segmento de path, o que faz o
`update_last_seen` dar sempre 404 em HTML. Nunca foi reportado a montante.

Hoje o merge não conflitua, porque o upstream não toca nessas linhas. Isso pode
mudar a qualquer momento, e um merge que remova a função é silencioso: compila,
passa nos testes locais, e só parte o read-sync em produção. Verificar sempre.

## Passos

### 1. Levantar o que há

```bash
git fetch upstream --tags
git rev-list --left-right --count main...upstream/main   # <à frente> <atrás>
git log --oneline main..upstream/main
git log --oneline upstream/main..main                    # o que é só nosso
```

### 2. Merge de ensaio num worktree descartável

Nunca fazer o merge real primeiro. O ensaio corre num worktree separado, portanto
a árvore de trabalho fica intacta se algo correr mal.

```bash
W=$(mktemp -d)/mergetest
git worktree add --detach "$W" main
cd "$W" && git merge --no-edit upstream/main
```

Conflitos aqui = parar e reportar quais os ficheiros, sem tentar resolver
sozinho.

### 3. Confirmar o delta e a saúde do build, no worktree

```bash
grep -q escapeChatwootPathSegment "$W/src/infrastructure/chatwoot/client.go" \
  && test -f "$W/src/infrastructure/chatwoot/escape_path_test.go" \
  && echo "delta preservado"
cd "$W/src" && go build ./... && go vet ./... && go test ./...
```

Se o delta desapareceu, **o merge não presta como está** — é preciso reaplicar a
função antes de o levar para o `main`.

### 4. Verificar o que muda em produção

Duas coisas mudam o perfil de risco do deploy que se segue:

```bash
# migrations: 43 antes e 43 depois = swap de binário reversível
for r in main upstream/main; do
  git show $r:src/infrastructure/chatstorage/sqlite_repository.go |
    awk '/^func \(r \*SQLiteRepository\) getMigrations/,/^}$/' | grep -c '`'
done   # dividir cada resultado por 2

# variáveis de config novas
git diff main upstream/main -- src/config/ src/.env.example
```

Uma variável nova com default seguro não obriga a nada. Uma que seja obrigatória
tem de entrar no `/etc/gowa/gowa.env` **antes** do deploy, senão o serviço não
arranca.

### 5. Limpar o ensaio e fazer o merge real

```bash
git worktree remove --force "$W"
git merge --no-edit upstream/main
cd src && go build ./... && go vet ./... && go test ./...
```

### 6. Reportar

Resumir por categoria (features, fixes, chore), indicar a versão nova em
`config/settings.go`, se o Go mínimo do `go.mod` subiu, e o estado das
migrations. **Perguntar antes de fazer push** e antes de deploy.

Se o `AGENTS.md` vier alterado do upstream, verificar se ainda descreve a
realidade — historicamente traz factos desatualizados (versão, contagem de
migrations, e um `src/views/` que já não existe).

## Branches do fork

- `origin/feat/mcp-oauth` — deu origem ao upstream #802, já absorvida. Candidata a apagar.
- `origin/gowa/v7` — legado, centenas de commits atrás.
