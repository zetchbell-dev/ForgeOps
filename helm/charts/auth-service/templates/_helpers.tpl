{{/*
Expand the name of the chart.
*/}}
{{- define "auth-service.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited
to this (by the DNS naming spec).
*/}}
{{- define "auth-service.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "auth-service.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "auth-service.labels" -}}
helm.sh/chart: {{ include "auth-service.chart" . }}
{{ include "auth-service.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "auth-service.selectorLabels" -}}
app.kubernetes.io/name: {{ include "auth-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "auth-service.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "auth-service.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Name of the Secret the chart consumes (either the user-supplied existing
one, or the one this chart creates for local/dev use).
*/}}
{{- define "auth-service.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else }}
{{- printf "%s-secret" (include "auth-service.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Name of the ConfigMap the chart creates for non-secret config.
*/}}
{{- define "auth-service.configMapName" -}}
{{- printf "%s-config" (include "auth-service.fullname" .) }}
{{- end }}

{{/*
Resolved image reference: repository:tag, defaulting tag to Chart.AppVersion.
*/}}
{{- define "auth-service.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
Shared env block: references the ConfigMap and Secret this chart wires up,
plus extraEnv. Used by both the Deployment container and the migration Job,
so the two never drift out of sync on how POSTGRES_DSN/JWT_SIGNING_KEY/etc.
are sourced.
*/}}
{{- define "auth-service.env" -}}
- name: PORT
  valueFrom:
    configMapKeyRef:
      name: {{ include "auth-service.configMapName" . }}
      key: PORT
- name: SHUTDOWN_TIMEOUT
  valueFrom:
    configMapKeyRef:
      name: {{ include "auth-service.configMapName" . }}
      key: SHUTDOWN_TIMEOUT
- name: REDIS_ADDR
  valueFrom:
    configMapKeyRef:
      name: {{ include "auth-service.configMapName" . }}
      key: REDIS_ADDR
- name: REDIS_DB
  valueFrom:
    configMapKeyRef:
      name: {{ include "auth-service.configMapName" . }}
      key: REDIS_DB
- name: JWT_ISSUER
  valueFrom:
    configMapKeyRef:
      name: {{ include "auth-service.configMapName" . }}
      key: JWT_ISSUER
- name: JWT_ACCESS_TOKEN_TTL
  valueFrom:
    configMapKeyRef:
      name: {{ include "auth-service.configMapName" . }}
      key: JWT_ACCESS_TOKEN_TTL
- name: BCRYPT_COST
  valueFrom:
    configMapKeyRef:
      name: {{ include "auth-service.configMapName" . }}
      key: BCRYPT_COST
- name: POSTGRES_DSN
  valueFrom:
    secretKeyRef:
      name: {{ include "auth-service.secretName" . }}
      key: {{ if .Values.secrets.existingSecret }}{{ .Values.secrets.existingSecretKeys.postgresDSN }}{{ else }}POSTGRES_DSN{{ end }}
- name: JWT_SIGNING_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "auth-service.secretName" . }}
      key: {{ if .Values.secrets.existingSecret }}{{ .Values.secrets.existingSecretKeys.jwtSigningKey }}{{ else }}JWT_SIGNING_KEY{{ end }}
- name: REDIS_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "auth-service.secretName" . }}
      key: {{ if .Values.secrets.existingSecret }}{{ .Values.secrets.existingSecretKeys.redisPassword }}{{ else }}REDIS_PASSWORD{{ end }}
      optional: true
{{- with .Values.extraEnv }}
{{- toYaml . | nindent 0 }}
{{- end }}
{{- end }}
