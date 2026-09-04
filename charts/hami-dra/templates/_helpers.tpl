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
{{- if eq (include "hami.dra.driver.fake.profile" .) "hygon" -}}
{{ include "hami.dra.driver.fake.defaultHygonConfig" . }}
{{- else -}}
{{ include "hami.dra.driver.fake.defaultNvidiaConfig" . }}
{{- end -}}
{{- end -}}

{{- define "hami.dra.driver.fake.profile" -}}
{{- $profile := .Values.drivers.fake.profile | default "nvidia" -}}
{{- if and (ne $profile "nvidia") (ne $profile "hygon") -}}
{{- fail (printf "drivers.fake.profile must be \"nvidia\" or \"hygon\", got %q" $profile) -}}
{{- end -}}
{{- $profile -}}
{{- end -}}

{{- define "hami.dra.driver.fake.deviceClassName" -}}
{{- if eq (include "hami.dra.driver.fake.profile" .) "hygon" -}}
{{- if or (not .Values.drivers.fake.deviceClassName) (eq .Values.drivers.fake.deviceClassName "fake-gpu.project-hami.io") -}}
{{- .Values.dcuDeviceClassName -}}
{{- else -}}
{{- .Values.drivers.fake.deviceClassName -}}
{{- end -}}
{{- else -}}
{{- .Values.drivers.fake.deviceClassName | default "fake-gpu.project-hami.io" -}}
{{- end -}}
{{- end -}}

{{- define "hami.dra.driver.fake.driverName" -}}
{{- if eq (include "hami.dra.driver.fake.profile" .) "hygon" -}}
{{- if or (not .Values.drivers.fake.driverName) (eq .Values.drivers.fake.driverName "fake.dra.hami.io") -}}
{{- .Values.dcuDraDriverName -}}
{{- else -}}
{{- .Values.drivers.fake.driverName -}}
{{- end -}}
{{- else -}}
{{- .Values.drivers.fake.driverName | default "fake.dra.hami.io" -}}
{{- end -}}
{{- end -}}

{{- define "hami.dra.driver.fake.deviceType" -}}
{{- if eq (include "hami.dra.driver.fake.profile" .) "hygon" -}}
{{- "dcu" -}}
{{- else -}}
{{- "hami-gpu" -}}
{{- end -}}
{{- end -}}

{{- define "hami.dra.driver.fake.defaultNvidiaConfig" -}}
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

{{- define "hami.dra.driver.fake.defaultHygonConfig" -}}
{{/*
  Fake DCU devices aligned with dra.hygon.com ResourceSlice from
  HYGON-AI/k8s-hcu-dra-driver (K100_AI):
  attributes: architecture/brand/productName/type/uuid
  capacity: cores=120, memory=65520Mi, slices=4 (max 4 vHCU per card)
*/}}
groups:
  - name: default-dcu
    devices:
{{- range $index := until 8 }}
      - name: dcu-0000-00-{{ printf "%02d" $index }}-0
        allowMultipleAllocations: true
        attributes:
          architecture:
            string: ""
          brand:
            string: ""
          productName:
            string: K100_AI
          type:
            string: dcu
          uuid:
            string: {{ printf "TPXS3000021%05d" $index }}
        capacity:
          cores:
            value: "120"
            requestPolicy:
              default: "120"
              validRange:
                max: "120"
                min: "0"
                step: "1"
          memory:
            value: 65520Mi
            requestPolicy:
              default: 65520Mi
              validRange:
                max: 65520Mi
                min: "0"
                step: 1Mi
          slices:
            value: "4"
            requestPolicy:
              default: "1"
              validRange:
                max: "4"
                min: "1"
                step: "1"
{{- end }}
{{- end -}}

{{- define "hami.dra.dcu.deviceClassName" -}}
{{- if .Values.drivers.dcu.deviceClassName -}}
{{- .Values.drivers.dcu.deviceClassName -}}
{{- else -}}
{{- .Values.dcuDeviceClassName -}}
{{- end -}}
{{- end -}}

{{- define "hami.dra.dcu.driverName" -}}
{{- if .Values.drivers.dcu.driverName -}}
{{- .Values.drivers.dcu.driverName -}}
{{- else -}}
{{- .Values.dcuDraDriverName -}}
{{- end -}}
{{- end -}}

{{- define "hami.dra.webhook.deviceClassName" -}}
{{- if and .Values.drivers.fake.enabled (not .Values.drivers.nvidia.enabled) -}}
{{- include "hami.dra.driver.fake.deviceClassName" . -}}
{{- else if eq (include "hami.dra.webhook.deviceVendor" .) "hygon" -}}
{{- include "hami.dra.dcu.deviceClassName" . -}}
{{- else -}}
{{- "hami-core-gpu.project-hami.io" -}}
{{- end -}}
{{- end -}}

{{- define "hami.dra.webhook.driverName" -}}
{{- if and .Values.drivers.fake.enabled (not .Values.drivers.nvidia.enabled) -}}
{{- include "hami.dra.driver.fake.driverName" . -}}
{{- else if eq (include "hami.dra.webhook.deviceVendor" .) "hygon" -}}
{{- include "hami.dra.dcu.driverName" . -}}
{{- else -}}
{{- "hami-core-gpu.project-hami.io" -}}
{{- end -}}
{{- end -}}

{{- define "hami.dra.webhook.deviceVendor" -}}
{{- .Values.deviceVendor | default "nvidia" -}}
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
