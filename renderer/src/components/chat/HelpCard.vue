<template>
  <div class="help-card">
    <div class="help-rule" aria-hidden="true">
      <span class="rule-line" />
      <span class="rule-cluster">
        <span class="tag">{{ t('help.tag') }}</span>
      </span>
      <span class="rule-line" />
    </div>

    <div class="help-body">
      <div class="body-rail" aria-hidden="true" />
      <div class="body-content">
        <table class="help-table">
          <thead>
            <tr>
              <th>{{ t('help.colCommand') }}</th>
              <th>{{ t('help.colDescription') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="cmd in rows" :key="cmd.id">
              <td class="cmd-cell">/{{ cmd.id }}</td>
              <td class="desc-cell">{{ cmd.description }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { builtinCommands } from '../../commands'
import { t } from '../../i18n'

const rows = computed(() =>
  builtinCommands.map((c) => ({
    id: c.id,
    description: t(c.descriptionKey),
  })),
)
</script>

<style scoped>
.help-card {
  margin: 14px 0;
  display: flex;
  flex-direction: column;
}

.help-rule {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 0;
  color: var(--ink-4);
}

.rule-line {
  flex: 1 1 auto;
  height: 0;
  border-top: 1px dashed var(--rule-soft);
}

.rule-cluster {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 3px 10px;
  background: var(--panel);
  border: 1px solid var(--rule-softer);
  border-radius: 2px;
}

.tag {
  font-family: var(--mono);
  font-size: 9.5px;
  font-weight: 600;
  letter-spacing: 0.18em;
  color: var(--violet);
}

.help-body {
  margin-top: 8px;
  display: flex;
  gap: 12px;
  animation: bieneFadeIn 0.18s ease both;
}

.body-rail {
  flex: 0 0 2px;
  align-self: stretch;
  background: var(--violet);
  opacity: 0.6;
}

.body-content {
  flex: 1 1 auto;
  min-width: 0;
  padding: 2px 0 6px;
}

.help-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  line-height: 1.55;
  color: var(--ink-2);
}

.help-table thead th {
  text-align: left;
  padding: 4px 12px 4px 0;
  font-family: var(--mono);
  font-size: 9.5px;
  font-weight: 600;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--ink-4);
  border-bottom: 1px dashed var(--rule-softer);
}

.help-table tbody td {
  padding: 6px 12px 6px 0;
  vertical-align: top;
  border-bottom: 1px dashed var(--rule-softer);
}

.help-table tbody tr:last-child td {
  border-bottom: none;
}

.cmd-cell {
  font-family: var(--mono);
  font-size: 12px;
  color: var(--violet);
  white-space: nowrap;
  width: 1%;
  padding-right: 16px !important;
}

.desc-cell {
  color: var(--ink-3);
}
</style>
