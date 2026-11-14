{{- define "gojellyfin.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "gojellyfin.fullname" -}}
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

{{- define "gojellyfin.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "gojellyfin.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gojellyfin.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{- define "gojellyfin.podLabels" -}}
{{- $selector := fromYaml (include "gojellyfin.selectorLabels" (dict "root" .root "component" .component)) }}
{{- toYaml (merge $selector (default dict .pod.labels)) }}
{{- end }}

{{- define "gojellyfin.serviceLabels" -}}
{{- $labels := fromYaml (include "gojellyfin.labels" (dict "root" .root "component" .component)) }}
{{- toYaml (merge $labels (default dict .root.Values.service.labels)) }}
{{- end }}

{{- define "gojellyfin.extraEnv" -}}
{{- with .Values.env }}
{{- toYaml . }}
{{ end }}
{{- end }}

{{- define "gojellyfin.labels" -}}
helm.sh/chart: {{ include "gojellyfin.chart" .root }}
{{ include "gojellyfin.selectorLabels" . }}
{{- with .root.Chart.AppVersion }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .root.Release.Service }}
{{- end }}

{{- define "gojellyfin.tag" -}}
{{- default .Chart.AppVersion .Values.image.tag }}
{{- end }}

{{- define "gojellyfin.image" -}}
{{- printf "%s:%s" .Values.image.repository (include "gojellyfin.tag" .) }}
{{- end }}

{{- define "gojellyfin.pullPolicy" -}}
{{- if .policy -}}
{{- .policy -}}
{{- else if eq (toString .tag) "latest" -}}
Always
{{- else -}}
IfNotPresent
{{- end -}}
{{- end }}

{{- define "gojellyfin.imagePullPolicy" -}}
{{- include "gojellyfin.pullPolicy" (dict "policy" .Values.image.pullPolicy "tag" (include "gojellyfin.tag" .)) }}
{{- end }}

{{- define "gojellyfin.podSpec" -}}
{{- range $field, $value := omit . "annotations" "labels" "containerSecurityContext" }}
{{- if or (kindIs "bool" $value) (kindIs "float64" $value) (kindIs "int64" $value) (kindIs "int" $value) $value }}
{{ $field }}:
{{- if or (kindIs "map" $value) (kindIs "slice" $value) }}
{{- toYaml $value | nindent 2 }}
{{- else }} {{ $value }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "gojellyfin.databaseSecretName" -}}
{{- default (include "gojellyfin.fullname" .) .Values.database.existingSecret }}
{{- end }}

{{- define "gojellyfin.databaseEnv" -}}
- name: DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: {{ include "gojellyfin.databaseSecretName" . }}
      key: {{ .Values.database.secretKey }}
{{- end }}

{{- define "gojellyfin.temporalEnv" -}}
{{- if .Values.temporal.hostPort }}
{{- if not .Values.temporal.namespace }}
{{- fail "temporal.hostPort is set but temporal.namespace is not, and the binary refuses to invent one" }}
{{- end }}
- name: TEMPORAL_HOSTPORT
  value: {{ .Values.temporal.hostPort | quote }}
- name: TEMPORAL_NAMESPACE
  value: {{ .Values.temporal.namespace | quote }}
{{- end }}
{{- end }}

{{- define "gojellyfin.sharedEnv" -}}
{{- $mounts := list .Values.media.mountPath }}
{{- range .Values.media.extraVolumes }}
{{- $mounts = append $mounts .mountPath }}
{{- end }}
{{- with (default $mounts .Values.media.directories) }}
- name: MEDIA_DIRECTORIES
  value: {{ join "," . | quote }}
{{- end }}
{{- with .Values.tmdb.existingSecret }}
- name: TMDB_API_KEY
  valueFrom:
    secretKeyRef:
      name: {{ . }}
      key: {{ $.Values.tmdb.secretKey }}
{{- end }}
{{- with .Values.tracing.otlpEndpoint }}
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: {{ . | quote }}
{{- end }}
{{- include "gojellyfin.temporalEnv" . }}
{{- end }}

{{- define "gojellyfin.workerEnv" -}}
{{- include "gojellyfin.extraEnv" . }}
{{- include "gojellyfin.databaseEnv" . }}
{{- include "gojellyfin.sharedEnv" . }}
{{- end }}

{{- define "gojellyfin.serverEnv" -}}
{{- include "gojellyfin.extraEnv" .root }}
{{- include "gojellyfin.databaseEnv" .root }}
- name: HTTP_PORT
  value: {{ .root.Values.httpPort | quote }}
{{- with .root.Values.hostname }}
- name: PUBLISHED_SERVER_URL
  value: {{ printf "%s://%s" $.root.Values.scheme . | quote }}
{{- end }}
{{- with .root.Values.serverId }}
- name: SERVER_ID
  value: {{ . | quote }}
{{- end }}
{{- if .root.Values.cors.enabled }}
{{- if not .root.Values.cors.origin }}
{{- fail "cors.enabled needs cors.origin: an empty origin reflects whatever origin asks, so the restriction would restrict nothing" }}
{{- end }}
- name: CORS_ORIGINS
  value: {{ .root.Values.cors.origin | quote }}
{{- end }}
{{- with .root.Values.trustedProxies }}
- name: TRUSTED_PROXIES
  value: {{ join "," . | quote }}
{{- end }}
{{- include "gojellyfin.oidcEnv" .root }}
- name: TRANSCODER_JOBS
  value: {{ include "gojellyfin.transcoderJobs" .workload | quote }}
- name: TRANSCODER_STALL_TIMEOUT
  value: {{ .workload.transcoderStallTimeout | quote }}
{{- include "gojellyfin.sharedEnv" .root }}
{{- end }}

{{- define "gojellyfin.oidcEnv" -}}
{{- with .Values.oidc }}
{{- if and .issuerUrl .clientId }}
{{- if not $.Values.hostname }}
{{- fail "oidc.issuerUrl needs hostname: the provider has to be told where to send the browser back to" }}
{{- end }}
- name: OIDC_ISSUER_URL
  value: {{ .issuerUrl | quote }}
- name: OIDC_CLIENT_ID
  value: {{ .clientId | quote }}
{{- with .existingSecret }}
- name: OIDC_CLIENT_SECRET
  valueFrom:
    secretKeyRef:
      name: {{ . }}
      key: {{ $.Values.oidc.secretKey }}
{{- end }}
{{- with .scopes }}
- name: OIDC_SCOPES
  value: {{ . | quote }}
{{- end }}
{{- with .usernameClaim }}
- name: OIDC_USERNAME_CLAIM
  value: {{ . | quote }}
{{- end }}
{{- with .groupsClaim }}
- name: OIDC_GROUPS_CLAIM
  value: {{ . | quote }}
{{- end }}
{{- with .adminGroup }}
- name: OIDC_ADMIN_GROUP
  value: {{ . | quote }}
{{- end }}
{{- with .allowedGroup }}
- name: OIDC_ALLOWED_GROUP
  value: {{ . | quote }}
{{- end }}
{{- if .linkExistingUsers }}
- name: OIDC_LINK_EXISTING_USERS
  value: "true"
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "gojellyfin.cpuCores" -}}
{{- $cpu := toString . -}}
{{- if hasSuffix "m" $cpu -}}
{{- divf (float64 (trimSuffix "m" $cpu)) 1000 -}}
{{- else -}}
{{- float64 $cpu -}}
{{- end -}}
{{- end }}

{{- define "gojellyfin.transcoderJobs" -}}
{{- if .transcoderJobs -}}
{{- .transcoderJobs -}}
{{- else -}}
{{- $cores := int (floor (float64 (include "gojellyfin.cpuCores" (dig "requests" "cpu" "1" (.resources | default dict))))) -}}
{{- max $cores 1 -}}
{{- end -}}
{{- end }}

{{- define "gojellyfin.mediaVolume" -}}
- name: media
{{- if .Values.media.volume }}
  {{- toYaml .Values.media.volume | nindent 2 }}
{{- else }}
  emptyDir: {}
{{- end }}
{{- range .Values.media.extraVolumes }}
- name: {{ .name }}
  {{- toYaml .volume | nindent 2 }}
{{- end }}
{{- end }}

{{- define "gojellyfin.mediaVolumeMount" -}}
- name: media
  mountPath: {{ .root.Values.media.mountPath }}
  readOnly: {{ .readOnly }}
{{- range .root.Values.media.extraVolumes }}
- name: {{ .name }}
  mountPath: {{ .mountPath }}
  readOnly: {{ $.readOnly }}
{{- end }}
{{- end }}
