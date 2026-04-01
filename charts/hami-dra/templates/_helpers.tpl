{{- define "hami.dra.webhook.fullname" -}}
{{- printf "%s-%s" (include "common.names.fullname" .) "webhook" | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{- define "hami.dra.webhook.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.webhook.image "global" .Values.global) }}
{{- end -}}

{{- define "hami.dra.webhook.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.webhook.image) "global" .Values.global) }}
{{- end -}}

{{- define "hami.dra.driver.nvidia.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.drivers.nvidia.image "global" .Values.global) }}
{{- end -}}

{{- define "hami.dra.driver.nvidia.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.drivers.nvidia.image) "global" .Values.global) }}
{{- end -}}

{{- define "hami.dra.driver.fake.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.drivers.fake.image "global" .Values.global) }}
{{- end -}}

{{- define "hami.dra.driver.fake.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.drivers.fake.image) "global" .Values.global) }}
{{- end -}}

{{- define "hami.dra.driver.fake.configMapName" -}}
{{- if .Values.drivers.fake.configMap.existingName -}}
{{- .Values.drivers.fake.configMap.existingName -}}
{{- else if .Values.drivers.fake.configMap.name -}}
{{- .Values.drivers.fake.configMap.name -}}
{{- else -}}
{{- printf "%s-%s" (include "common.names.fullname" .) "fake-dra-config" | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "hami.dra.driver.fake.defaultConfig" -}}
groups:
  - name: default-a100
    devices:
{{- range $index := until 8 }}
      - name: gpu-{{ $index }}
        allowMultipleAllocations: true
        attributes:
          architecture:
            string: Ampere
          attr.project-hami.io/minor:
            int: {{ $index }}
          brand:
            string: Nvidia
          cudaComputeCapability:
            version: 8.0.0
          cudaDriverVersion:
            version: 12.9.0
          driverVersion:
            version: 575.57.8
          minor:
            int: {{ $index }}
          pcieBusID:
            string: {{ printf "0000:%02x:00.0" (add 97 $index) }}
          productName:
            string: NVIDIA A100-SXM4-80GB
          resource.kubernetes.io/pcieRoot:
            string: pci0000:5a
          type:
            string: hami-gpu
          uuid:
            string: {{ printf "GPU-00000000-0000-0000-0000-%012d" $index }}
        capacity:
          cores:
            value: "100"
            requestPolicy:
              default: "100"
              validRange:
                max: "100"
                min: "0"
                step: "1"
          memory:
            value: 80Gi
            requestPolicy:
              default: 80Gi
              validRange:
                max: 80Gi
                min: 1Mi
                step: 1Mi
{{- end }}
{{- end -}}

{{- define "hami.dra.webhook.deviceClassName" -}}
{{- if .Values.webhook.dra.deviceClassName -}}
{{- .Values.webhook.dra.deviceClassName -}}
{{- else if and .Values.drivers.fake.enabled (not .Values.drivers.nvidia.enabled) -}}
{{- .Values.drivers.fake.deviceClassName -}}
{{- else -}}
{{- "hami-core-gpu.project-hami.io" -}}
{{- end -}}
{{- end -}}

{{- define "hami.dra.webhook.driverName" -}}
{{- if .Values.webhook.dra.driverName -}}
{{- .Values.webhook.dra.driverName -}}
{{- else if and .Values.drivers.fake.enabled (not .Values.drivers.nvidia.enabled) -}}
{{- .Values.drivers.fake.driverName -}}
{{- else -}}
{{- "hami-core-gpu.project-hami.io" -}}
{{- end -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "hami-dra-webhook.labels" -}}
helm.sh/chart: {{ .Chart.Name }}
{{ include "hami-dra-webhook.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "hami-dra-webhook.selectorLabels" -}}
app.kubernetes.io/name: {{ .Release.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: webhook
{{- end }}

{{- define "hami.dra.monitor.fullname" -}}
{{- printf "%s-%s" (include "common.names.fullname" .) "monitor" | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{- define "hami.dra.monitor.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.monitor.image "global" .Values.global) }}
{{- end -}}

{{- define "hami.dra.monitor.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.monitor.image) "global" .Values.global) }}
{{- end -}}

{{/*
Common labels for monitor
*/}}
{{- define "hami-dra-monitor.labels" -}}
helm.sh/chart: {{ .Chart.Name }}
{{ include "hami-dra-monitor.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels for monitor
*/}}
{{- define "hami-dra-monitor.selectorLabels" -}}
app.kubernetes.io/name: {{ .Release.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: monitor
{{- end }}
