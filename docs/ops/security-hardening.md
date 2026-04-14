# Безопасность и hardening развёртывания

Документ описывает параметры и практики усиления безопасности при эксплуатации Animus Datalab, что снижает риск несоответствия комплаенсу.

## 1. RBAC и аутентификация

**OIDC (через Gateway, Control Plane):**
- Параметры в `values-datapilot.yaml`, секция `oidc.*`.
- Режим включается через `auth.mode=oidc`.

**Пример:**
```yaml
auth:
  mode: oidc
  internalAuthSecret: "<shared-secret>"

oidc:
  issuerURL: "https://idp.example.local/realms/animus"
  clientID: "animus"
  clientSecret: "<client-secret>"
  redirectURL: "https://gateway.example.local/auth/callback"
  rolesClaim: roles
```

**SAML:**
- Если поддерживается вашим Gateway‑развёртыванием, требуется отдельная конфигурация IdP и маршрутов.
- Рекомендуется закрепить параметры в отдельном values‑файле и явно документировать их для среды.

## 2. Секреты

**Принцип:** значения секретов выдаются только DP на время исполнения.

**Vault (Kubernetes auth):**
```yaml
secrets:
  provider: vault
  vault:
    addr: "https://vault.example.local"
    role: "animus-dataplane"
    authPath: "auth/kubernetes/login"
    jwtPath: "/var/run/secrets/kubernetes.io/serviceaccount/token"
```

**Примечания:**
- Не включайте `secrets.staticJSON` в production.
- Проверьте аудит событий доступа к секретам.

## 3. Egress‑политика DP

**Рекомендация:** deny‑by‑default с явными allowlist для SCM/S3/Vault.

**Пример NetworkPolicy (общий шаблон):**
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: animus-dataplane-egress
  namespace: animus-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: dataplane
  policyTypes: ["Egress"]
  egress:
    - to:
        - namespaceSelector: {}
      ports:
        - protocol: TCP
          port: 443
```

**Примечание:** конкретные allowlist адресов/портов задаются согласно требованиям безопасности и используемым интеграциям.

## 4. Аудит и экспорт

**Настройки экспортера:**
- `AUDIT_EXPORT_DESTINATION` — `webhook` или `syslog`.
- `AUDIT_EXPORT_WEBHOOK_URL` или `AUDIT_EXPORT_SYSLOG_ADDR`.

**Принцип:** payload не должен содержать секреты.

## 5. Retention и legal hold

- Настраиваются политиками retention на уровне доменных сущностей.
- Удаление при legal hold блокируется и аудируется.

## 6. Диагностика

```bash
kubectl -n animus-system logs deploy/animus-datapilot-gateway --tail=200
kubectl -n animus-system logs deploy/animus-dataplane --tail=200
```
