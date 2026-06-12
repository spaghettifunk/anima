{{/*
Expand the name of the chart.
*/}}
{{- define "anima.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "anima.fullname" -}}
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
{{- define "anima.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a component-specific resource name.
*/}}
{{- define "anima.componentName" -}}
{{- printf "%s-%s" (include "anima.fullname" .root) .component | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels shared by all resources.
*/}}
{{- define "anima.labels" -}}
helm.sh/chart: {{ include "anima.chart" . }}
app.kubernetes.io/name: {{ include "anima.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels for one component.
*/}}
{{- define "anima.selectorLabels" -}}
app.kubernetes.io/name: {{ include "anima.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "anima.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "anima.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the config Secret name.
*/}}
{{- define "anima.configSecretName" -}}
{{- default (printf "%s-config" (include "anima.fullname" .)) .Values.config.existingSecret | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Render the application config.toml.
*/}}
{{- define "anima.configToml" -}}
[collector]
address = {{ .Values.config.collector.address | quote }}
port = {{ .Values.config.collector.port }}
metrics_topic = {{ .Values.config.collector.metricsTopic | quote }}
traces_topic = {{ .Values.config.collector.tracesTopic | quote }}
logs_topic = {{ .Values.config.collector.logsTopic | quote }}

[producer]
brokers = {{ .Values.config.producer.brokers | quote }}
partitions_topic = {{ .Values.config.producer.partitionsPerTopic }}

[consumer]
brokers = {{ .Values.config.consumer.brokers | quote }}
group = {{ .Values.config.consumer.group | quote }}
batch_size = {{ .Values.config.consumer.batchSize }}
flush_interval_seconds = {{ .Values.config.consumer.flushIntervalSeconds }}
partitions_topic = {{ .Values.config.consumer.partitionsPerTopic }}

[store]
address = {{ .Values.config.store.address | quote }}
port = {{ .Values.config.store.port }}
dbname = {{ .Values.config.store.dbname | quote }}
username = {{ .Values.config.store.username | quote }}
password = {{ .Values.config.store.password | quote }}
tls = {{ .Values.config.store.tls }}

[sources]
tenant = {{ .Values.config.sources.tenant | quote }}
rules_url = {{ .Values.config.sources.rulesURL | quote }}
grafana_url = {{ .Values.config.sources.grafanaURL | quote }}
grafana_token = {{ .Values.config.sources.grafanaToken | quote }}
grafana_timeout_seconds = {{ .Values.config.sources.grafanaTimeoutSeconds }}
query_history_enabled = {{ .Values.config.sources.queryHistoryEnabled }}
query_history_datasource_uid = {{ .Values.config.sources.queryHistoryDatasourceUID | quote }}
query_history_lookback_hours = {{ .Values.config.sources.queryHistoryLookbackHours }}
query_history_limit = {{ .Values.config.sources.queryHistoryLimit }}
query_history_sort = {{ .Values.config.sources.queryHistorySort | quote }}
query_history_max_pages = {{ .Values.config.sources.queryHistoryMaxPages }}

[reporter]
address = {{ .Values.config.reporter.address | quote }}
port = {{ .Values.config.reporter.port }}
default_tenant = {{ .Values.config.reporter.defaultTenant | quote }}
default_lookback_days = {{ .Values.config.reporter.defaultLookbackDays }}
default_limit = {{ .Values.config.reporter.defaultLimit }}
max_limit = {{ .Values.config.reporter.maxLimit }}
max_queried_at_entries = {{ .Values.config.reporter.maxQueriedAtEntries }}

[query_audit_proxy]
address = {{ .Values.config.queryAuditProxy.address | quote }}
port = {{ .Values.config.queryAuditProxy.port }}
upstream = {{ .Values.config.queryAuditProxy.upstream | quote }}
log_raw_query = {{ .Values.config.queryAuditProxy.logRawQuery }}
event_buffer_size = {{ .Values.config.queryAuditProxy.eventBufferSize }}
audit_topic = {{ .Values.config.queryAuditProxy.auditTopic | quote }}

[logger]
level = {{ .Values.config.logger.level | quote }}
{{- end }}

{{/*
Render the common pod spec shared by Deployments and CronJobs.
*/}}
{{- define "anima.podSpec" -}}
{{- $root := .root -}}
{{- $componentName := .componentName -}}
{{- $component := .component -}}
{{- with $root.Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
serviceAccountName: {{ include "anima.serviceAccountName" $root }}
{{- with $root.Values.podSecurityContext }}
securityContext:
  {{- toYaml . | nindent 2 }}
{{- end }}
containers:
  - name: {{ $componentName }}
    {{- with $root.Values.securityContext }}
    securityContext:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    image: "{{ $root.Values.image.repository }}:{{ $root.Values.image.tag | default $root.Chart.AppVersion }}"
    imagePullPolicy: {{ $root.Values.image.pullPolicy }}
    {{- with $root.Values.workingDir }}
    workingDir: {{ . | quote }}
    {{- end }}
    {{- with $component.command }}
    command:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    args:
      {{- toYaml ($component.args | default (list $componentName)) | nindent 6 }}
    {{- with $component.env }}
    env:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- with $component.envFrom }}
    envFrom:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- if $component.containerPort }}
    ports:
      - name: http
        containerPort: {{ $component.containerPort }}
        protocol: TCP
    {{- end }}
    {{- with $component.livenessProbe }}
    livenessProbe:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- with $component.readinessProbe }}
    readinessProbe:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- with $component.resources }}
    resources:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    volumeMounts:
      - name: anima-config
        mountPath: {{ $root.Values.config.mountPath | quote }}
        subPath: config.toml
        readOnly: true
      {{- with $root.Values.volumeMounts }}
      {{- toYaml . | nindent 6 }}
      {{- end }}
      {{- with $component.volumeMounts }}
      {{- toYaml . | nindent 6 }}
      {{- end }}
volumes:
  - name: anima-config
    secret:
      secretName: {{ include "anima.configSecretName" $root }}
  {{- with $root.Values.volumes }}
  {{- toYaml . | nindent 2 }}
  {{- end }}
  {{- with $component.volumes }}
  {{- toYaml . | nindent 2 }}
  {{- end }}
{{- with $root.Values.nodeSelector }}
nodeSelector:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with $root.Values.affinity }}
affinity:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with $root.Values.tolerations }}
tolerations:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}
