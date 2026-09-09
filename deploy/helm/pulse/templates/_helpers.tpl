{{- define "pulse.labels" -}}
app.kubernetes.io/name: pulse
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: helm
{{- end }}

{{- define "pulse.env" -}}
- name: DATABASE_URL
  valueFrom: { secretKeyRef: { name: pulse-secrets, key: DATABASE_URL } }
- name: REDIS_URL
  valueFrom: { secretKeyRef: { name: pulse-secrets, key: REDIS_URL } }
- name: JWT_SECRET
  valueFrom: { secretKeyRef: { name: pulse-secrets, key: JWT_SECRET } }
- name: METRICS_PORT
  value: {{ .Values.metricsPort | quote }}
{{- end }}
