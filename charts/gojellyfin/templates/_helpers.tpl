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

{{- define "gojellyfin.labels" -}}
helm.sh/chart: {{ include "gojellyfin.chart" .root }}
{{ include "gojellyfin.selectorLabels" . }}
{{- with .root.Chart.AppVersion }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .root.Release.Service }}
{{- end }}

{{- define "gojellyfin.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) }}
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

{{- define "gojellyfin.workerEnv" -}}
{{- include "gojellyfin.databaseEnv" . }}
{{- include "gojellyfin.temporalEnv" . }}
{{- end }}

{{- define "gojellyfin.serverEnv" -}}
{{- include "gojellyfin.checkTranscoderJobs" . }}
{{- include "gojellyfin.databaseEnv" .root }}
- name: HTTP_PORT
  value: {{ .root.Values.httpPort | quote }}
{{- with .root.Values.publishedServerURL }}
- name: PUBLISHED_SERVER_URL
  value: {{ . | quote }}
{{- end }}
{{- with .root.Values.corsOrigins }}
- name: CORS_ORIGINS
  value: {{ join "," . | quote }}
{{- end }}
- name: TRANSCODER_JOBS
  value: {{ .workload.transcoderJobs | quote }}
- name: TRANSCODER_STALL_TIMEOUT
  value: {{ .workload.transcoderStallTimeout | quote }}
{{- include "gojellyfin.temporalEnv" .root }}
{{- end }}

{{- define "gojellyfin.cpuCores" -}}
{{- $cpu := toString . -}}
{{- if hasSuffix "m" $cpu -}}
{{- divf (float64 (trimSuffix "m" $cpu)) 1000 -}}
{{- else -}}
{{- float64 $cpu -}}
{{- end -}}
{{- end }}

{{- define "gojellyfin.checkTranscoderJobs" -}}
{{- $limit := dig "limits" "cpu" "" .workload.resources -}}
{{- if $limit -}}
{{- if gt (float64 .workload.transcoderJobs) (float64 (include "gojellyfin.cpuCores" $limit)) -}}
{{- fail (printf "%s.transcoderJobs is %v with a cpu limit of %v: one encode saturates about a core, so a pod that accepts more than it can run makes every stream on it slower without finishing any sooner" .component .workload.transcoderJobs $limit) -}}
{{- end -}}
{{- end -}}
{{- end }}

{{- define "gojellyfin.mediaClaimName" -}}
{{- default (printf "%s-media" (include "gojellyfin.fullname" .)) .Values.media.existingClaim }}
{{- end }}

{{- define "gojellyfin.mediaVolume" -}}
- name: media
  persistentVolumeClaim:
    claimName: {{ include "gojellyfin.mediaClaimName" .root }}
{{- end }}

{{- define "gojellyfin.mediaVolumeMount" -}}
- name: media
  mountPath: {{ .root.Values.media.mountPath }}
  readOnly: {{ .workload.mediaReadOnly }}
{{- end }}
