# Briefing Técnico: Suporte a Templates de Negócio e Mídia de Cabeçalho (Issue #9)

## 1. Contexto e Problema

- **Issue de Referência:** [GOWA-ATCode/go-whatsapp-web-multidevice#9](https://github.com/GOWA-ATCode/go-whatsapp-web-multidevice/issues/9)
- **Sintoma Observado:** Mensagens de template enviadas via WhatsApp Business Cloud API por terceiros (exemplo: boleto em PDF da ACE Guarulhos com texto no corpo e rodapé) chegavam aos consumidores de webhook (Fazz) com apenas o envelope básico do chat/remetente, sem `body`, sem `template` e sem arquivo anexo (`document`). No Fazz a mensagem aparecia com placeholder vazio ("Mensagem").
- **Causa Raiz:**
  1. O binário em execução no LXC 139 rodava o commit `e94c82a`, anterior ao upstream [#857](https://github.com/aldinokemal/go-whatsapp-web-multidevice/pull/857) (`35d2e46`, lançado na v9.6.0).
  2. Mensagens de template chegam via protobuf como `TemplateMessage` (`HydratedFourRowTemplate` ou `InteractiveMessageTemplate`), sem campos comuns `Conversation` ou `ExtendedTextMessage`. Na versão antiga do código, `ExtractMessageTextFromProto` retornava string vazia e `buildOtherMessageTypes` ignorava templates, omitindo todo o conteúdo no webhook.
  3. **A lacuna remanescente mesmo com o upstream #857:** O upstream #857 implementou a renderização textual (`body` achatado e objeto `template` estruturado), mas **não extrai nem faz download do arquivo de mídia do cabeçalho**. O próprio autor do upstream (@aldinokemal) registrou na descrição do PR #857:
     > *"Known follow-ups, not addressed here: ... Template and buttons header media is not stored."*
     E na documentação oficial (`docs/webhook-payload.md`):
     > *"header_type is set when the header is media: image, video, document, location or product. The header media itself is not included."*
     Com isso, mesmo no upstream v9.6.0, o PDF do boleto não chegava ao webhook nem ao Chatwoot.

---

## 2. O que foi Feito

### 2.1 Merge do Upstream (`aldinokemal/main` até v9.6.0 + fixes subsequentes)
Trouxemos 13 commits do upstream até o commit `3971c4c`:
- **#857 (`35d2e46`):** Suporte para mensagens de WhatsApp Business (templates, botões, listas, produtos, pedidos).
- **#851 / #856 (`13c55ff`):** Correção de QR code stale no modal de login.
- **#858 (`dc6d11a`):** Suporte a vCards de iOS com propriedades `TEL` prefixadas por grupos (`item1.TEL`).
- **#861 (`1df3866`):** Preservação de link preview rico na edição de mensagens (`POST /message/{id}/update` e MCP).
- **#862 (`767742b`):** Suporte ao reenvio/reshare de status posts.
- **#863 (`dcb962d`):** Persistência do contexto de resposta (`reply context`) no chat storage em live e history sync.
- **#864 (`3971c4c`):** Encaminhamento de recibos de leitura emitidos por outros dispositivos conectados da própria conta (WhatsApp Web/Desktop).

*Conflitos de merge resolvidos:*
- `chatwoot_content_test.go`, `event_message.go`, `webhook_forward.go` e `whatsapp.go`: mantivemos e integramos todas as funcionalidades exclusivas da nossa org (renderização de formulários WhatsApp Flow, botões com Pix/`order_details`, campos verbatim de botões e `sender_contact_name`).

### 2.2 Incorporação de Melhorias do Fork Pessoal (`aguinaldotupy`)
- Cherry-pick dos commits de documentação OpenAPI e notas de usecase da branch `fix/avatar-not-set-vs-hidden` (PR [#859](https://github.com/aldinokemal/go-whatsapp-web-multidevice/pull/859)):
  - `1a7818b`: `docs(openapi): show the runtime codes for /user/avatar 403 and 404`
  - `5d3877e`: `docs(openapi): add ErrorForbidden for the /user/avatar 403`
  - `fc4bf7c`: `docs(user): explain why a nil avatar stays out of AVATAR_NOT_SET`

### 2.3 Implementação: Extração de Mídia de Cabeçalho de Templates
Commit: `bf047f7` (`feat(webhook): extract and expose header media from business template messages`)

1. **Helper de Extração de Mídia (`pkg/utils/business_message.go`):**
   - Função `ExtractHeaderMedia(msg *waE2E.Message) (doc *waE2E.DocumentMessage, img *waE2E.ImageMessage, vid *waE2E.VideoMessage)`: inspeciona `HydratedFourRowTemplate`, `InteractiveMessageTemplate`, `ButtonsMessage` e `InteractiveMessage` não-carrossel para extrair o objeto de mídia do cabeçalho.
2. **Download e Envio no Webhook (`infrastructure/whatsapp/event_message.go`):**
   - Em `buildMediaFields`, faz fallback para a mídia de cabeçalho quando a mensagem não tem mídia no nível raiz.
   - Preenche `payload["document"]`, `payload["image"]` ou `payload["video"]` com a estrutura padrão (caminho em disco quando auto-download estiver ativo, ou mapa `{"url", "filename"}`).
   - O Fazz e o Chatwoot recebem a mídia diretamente pelas chaves nativas sem necessitar de tratamentos especiais.
3. **Persistência no Chat Storage (`pkg/utils/whatsapp.go`):**
   - Atualizados `ExtractMediaInfo` e `ExtractMediaCaption` para também ler mídias de cabeçalho, garantindo que o SQLite registre o tipo e metadados de arquivo.
4. **Documentação (`docs/webhook-payload.md`):**
   - Atualizado para descrever que o arquivo de mídia de templates agora é extraído e exposto em `payload.document`, `payload.image` ou `payload.video`.
5. **Testes Unitários:**
   - Adicionado `TestBuildEventPayloadTemplateWithDocumentHeader` em `infrastructure/whatsapp/event_business_message_test.go`.
   - Adicionados casos `TemplateDocument` e `InteractiveImage` em `pkg/utils/whatsapp_test.go`.
   - Toda a suíte de testes (`go test ./...`) e validações de linter (`go vet ./...`) estão passando 100%.

---

## 3. Estrutura das Branches

### Repositório Atual (`GOWA-ATCode/go-whatsapp-web-multidevice`)
- **Branch `main`:**
  - Contém o merge do upstream `v9.6.0`, docs de avatar e a feature `bf047f7`.
  - Pronta para deploy. Ao realizar `git push origin main`, o workflow [`.github/workflows/deploy.yml`](file:///home/tupy/Projects/gowa/.github/workflows/deploy.yml) executa compilação e deploy automático no LXC 139 via Tailscale.

### Branch Isolada para PR ao Upstream
- **Branch local `feat/template-header-media`:**
  - Baseada diretamente em `upstream/main` puro (`3971c4c`).
  - Contém o commit limpo `64ebf70` com a feature e os testes.
  - Testada e validada contra o upstream sem nenhuma dependência das alterações específicas do GOWA-ATCode.

---

## 4. Próximos Passos Recomendados

1. **Deploy no LXC 139 (Produção da Organização):**
   ```sh
   git push origin main
   ```
   Acompanhar o workflow de deploy no GitHub Actions e verificar o recebimento de novos boletos no webhook do Fazz.

2. **Abrir PR no Repositório Principal (`aldinokemal/go-whatsapp-web-multidevice`):**
   - Enviar a branch local para o fork pessoal:
     ```sh
     git push fork feat/template-header-media
     ```
   - Abrir o Pull Request contra `aldinokemal/go-whatsapp-web-multidevice:main`.
   - Na descrição do PR, referenciar o PR #857 e citar que resolve o follow-up pendente *"Template and buttons header media is not stored"*.

3. **Status das outras PRs abertas no Upstream:**
   - **PR [#859](https://github.com/aldinokemal/go-whatsapp-web-multidevice/pull/859) (Avatar 403/404):** O Aldino comentou sobre um detalhe no retry de comunidade (`IsCommunity: true -> false`), onde o erro 401 pode não significar `AVATAR_HIDDEN`. Vale revisar esse ponto na branch `fix/avatar-not-set-vs-hidden` do fork pessoal.
   - **PR [#822](https://github.com/aldinokemal/go-whatsapp-web-multidevice/pull/822) (Chatwoot history sync reopen):** A branch `fix/chatwoot-history-sync-reopens-conversations` no fork pessoal contém a fila de retry durável no SQLite solicitada no review do Aldino, pendente de finalização.
