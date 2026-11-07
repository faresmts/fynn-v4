# Implantação e recuperação

Os arquivos de `deploy/` são modelos revisáveis; nenhuma VPS, domínio, certificado, serviço systemd ou armazenamento externo foi configurado automaticamente.

## Antes da primeira implantação

1. Escolher instância e arquitetura na Oracle; conferir elegibilidade, cotas e custos no console antes de provisionar. Não presumir disponibilidade da camada gratuita.
2. Criar usuário de serviço `financeiro`, diretório `/var/lib/meu-financeiro` com permissão 0700 e releases em `/opt/meu-financeiro/releases/<versão>`. O link `current` aponta para o release ativo; o banco fica fora dele.
3. Gerar binário para a arquitetura alvo (`GOOS=linux GOARCH=arm64` para ARM) e frontend em ambiente de build; copiar artefatos, scripts e checksums para o release.
4. Adaptar `.env.example` para `/etc/meu-financeiro.env`, permissão 0600, e provisionar as duas contas localmente como usuário de serviço. Não incluir senha no arquivo permanente.
5. Adaptar `deploy/nginx.conf` ao domínio e aos certificados existentes. O Go escuta em loopback; Nginx termina TLS. O frontend e a API têm a mesma origem.
6. Instalar `deploy/meu-financeiro.service`; habilitar somente após conferir os caminhos. Não usar `--demo` em produção. O servidor recusa banco com os usuários da demonstração sem esse modo.
7. Testar `/api/health`, login de cada pessoa, criação de R$ 0,01, saldo, cancelamento e isolamento entre usuários. Criar apenas dados fictícios na validação anterior ao uso real.

A configuração atual de limite de login usa o IP da conexão direta. Atrás de Nginx todas as tentativas compartilham o IP de loopback. Isso é conservador para duas pessoas, mas requer política de proxy confiável antes de ampliar o uso. `X-Forwarded-For` não é confiado implicitamente.

## Backup

`snapshot.py` utiliza `sqlite3.Connection.backup`, que usa a Online Backup API; nunca copia apenas o `.db` ativo em WAL. O snapshot é validado com `integrity_check` e `foreign_key_check`.

`backup.py` exige o binário **age**, um destinatário público age e um diretório externo já montado. A saída é criptografada antes de ser copiada ao destino; o checksum é verificado após a gravação. A chave privada fica fora do servidor. O manifesto contém versão, horário, checksum e contagens, sem descrições financeiras.

```sh
python3 deploy/backup.py \
  --database /var/lib/meu-financeiro/financeiro.db \
  --destination /mnt/financeiro-backup \
  --recipient 'age1SUBSTITUA_PELO_DESTINATARIO_REAL'
```

O diretório deve estar efetivamente em armazenamento externo; o script não consegue inferir se um caminho reside na mesma VPS. O unit de exemplo usa `RequiresMountsFor`, mas o administrador deve configurar/verificar o mount correspondente. Defina `AGE_RECIPIENT` em `/etc/meu-financeiro-backup.env` e adapte os units antes de ativar o timer.

Não ativar a rotina sem ensaio. Permanecem pendentes: retenção 7/4/6, confirmação de upload em destino não montado, alerta por idade/falha e integração de monitoramento. Falha de `age` ou cópia resulta em saída não zero; o systemd registra o erro. Um arquivo parcial não deve ser considerado válido: confira o manifesto e o checksum.

## Restauração isolada

1. Parar a aplicação do ambiente de recuperação e preservar o banco atual; nunca sobrescrever um banco ativo.
2. Copiar o `.db.age` e o manifesto de uma cópia externa. Conferir `encrypted_sha256` antes de descriptografar.
3. Em diretório restrito, executar `age -d -i <arquivo-da-chave> -o restaurado.db <backup.db.age>`. Manter a chave fora dos logs e do repositório.
4. Conferir `sha256` do banco contra o manifesto. Usar Python para `PRAGMA integrity_check` e `PRAGMA foreign_key_check` (a função `validate` em `snapshot.py` faz ambos).
5. Comparar versão do esquema e contagens; conferir saldos, lançamentos, contas e relações com os controles da cópia. **Excluir as sessões restauradas** (`DELETE FROM sessions`) para exigir nova autenticação.
6. Aplicar proprietário e modo 0600 ao banco, iniciar versão compatível em loopback e repetir os testes financeiros e de privacidade.
7. Medir duração e registrar perda máxima desde o backup. Liberar tráfego só depois da validação.

O teste automatizado verifica restauração local de snapshot em WAL. Criptografia, destino externo, retenção, restauração externa integral, RPO/RTO e falta de disco **ainda não foram ensaiados** neste ambiente.

## Releases e rollback

Executar testes e backup verificado antes de migrar. Guardar release anterior e checksum. O esquema atual é versão 1, criado atomicamente ao iniciar; versões desconhecidas maiores são recusadas. Downgrade de binário não desfaz migrações. Em futuras mudanças incompatíveis, avaliar dados posteriores antes de restaurar cópia em manutenção.

## Referências de implementação

- [Svelte — documentação](https://svelte.dev/docs/svelte/overview)
- [Driver SQLite modernc](https://pkg.go.dev/modernc.org/sqlite)
- [Argon2 em Go](https://pkg.go.dev/golang.org/x/crypto/argon2)
