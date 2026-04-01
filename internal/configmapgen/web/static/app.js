"use strict";

const translations = {
  en: {
    pageTitle: "Fake DRA ConfigMap Generator",
    errorLoadDefaults: "Failed to load default configuration",
    hero: {
      eyebrow: "Fake DRA",
      title: "ConfigMap Generator",
      subtitle: "Visually manage multiple node groups and per-node overrides. Key fields stay visible, non-critical fields like UUID, minor, and PCIe values are generated automatically, and the raw config remains editable when needed."
    },
    language: {
      label: "Language"
    },
    actions: {
      copyConfigMap: "Copy ConfigMap",
      reset: "Reset",
      regenerate: "Regenerate from form",
      copyRaw: "Copy raw config",
      remove: "Remove"
    },
    global: {
      title: "Global Settings",
      description: "Configure ConfigMap metadata and the global node selector here. Use the sections below to add or remove groups and node overrides visually.",
      namespace: "ConfigMap Namespace",
      name: "ConfigMap Name",
      key: "Data Key",
      nodeSelectorKey: "Global Node Selector Key",
      nodeSelectorValue: "Global Node Selector Value"
    },
    groups: {
      title: "Groups",
      description: "Each group matches a set of nodes by selector and generates a batch of devices.",
      add: "Add Group",
      empty: "No groups yet. Click the button above to add one.",
      unnamed: "Unnamed Group",
      summary: "selector: {selector} · {count} devices"
    },
    nodes: {
      title: "Node Overrides",
      description: "Define or override devices by node name. Devices with the same name override fields from matching groups.",
      add: "Add Node",
      empty: "No node overrides yet. Click the button above to add one.",
      unnamed: "Unnamed Node",
      summary: "node: {node} · {count} override devices"
    },
    tips: {
      generated: "Auto-generated: <code>uuid</code>, <code>minor</code>, <code>attr.project-hami.io/minor</code>, <code>pcieBusID</code>",
      preserved: "Preserved by default: <code>requestPolicy</code>, <code>driverVersion</code>, and CUDA-related fields"
    },
    raw: {
      title: "Raw Config",
      description: "This is the content of <code>data.config.yaml</code>. The visual editor generates it, but you can fine-tune it directly and the ConfigMap on the right updates in real time."
    },
    output: {
      title: "Rendered ConfigMap",
      description: "Ready for <code>kubectl apply -f</code>."
    },
    status: {
      generated: "Generated from form",
      dirty: "Raw config edited manually"
    },
    placeholders: {
      emptyLabelValue: "Leave empty to match labels with an empty value"
    },
    fields: {
      groupName: "Group Name",
      labelKey: "Label Key",
      labelValue: "Label Value",
      deviceCount: "Device Count",
      deviceNamePrefix: "Device Name Prefix",
      minorStart: "Starting Minor",
      productName: "GPU Model",
      memory: "Memory",
      cores: "Cores",
      advanced: "Advanced Settings",
      pcieBusStart: "Starting PCIe Bus (hex)",
      allowMultipleAllocations: "Enable `allowMultipleAllocations` automatically",
      nodeName: "Node Name"
    },
    copySuccess: {
      raw: "Raw config copied",
      configMap: "ConfigMap copied"
    }
  },
  "zh-CN": {
    pageTitle: "Fake DRA ConfigMap 生成器",
    errorLoadDefaults: "加载默认配置失败",
    hero: {
      eyebrow: "Fake DRA",
      title: "ConfigMap 生成器",
      subtitle: "可视化维护多个批量分组和多个节点覆盖。关键字段直出，UUID、minor、PCIe 等非关键项自动生成；需要细调时可直接改原始配置。"
    },
    language: {
      label: "语言"
    },
    actions: {
      copyConfigMap: "复制 ConfigMap",
      reset: "恢复默认",
      regenerate: "重新根据表单生成",
      copyRaw: "复制原始配置",
      remove: "删除"
    },
    global: {
      title: "全局配置",
      description: "这里放 ConfigMap 元信息和全局节点选择，下面可视化增删多个 group 和 node override。",
      namespace: "ConfigMap Namespace",
      name: "ConfigMap Name",
      key: "Data Key",
      nodeSelectorKey: "全局节点选择 Key",
      nodeSelectorValue: "全局节点选择 Value"
    },
    groups: {
      title: "批量分组 Groups",
      description: "每个 group 通过 selector 匹配一批节点，并批量生成一组设备。",
      add: "新增 Group",
      empty: "还没有 Group，点击上方按钮新增。",
      unnamed: "未命名 Group",
      summary: "selector: {selector} · {count} 个设备"
    },
    nodes: {
      title: "节点覆盖 Nodes",
      description: "按节点名精确定义或覆盖设备，同名设备会覆盖 group 里对应字段。",
      add: "新增 Node",
      empty: "还没有 Node 覆盖，点击上方按钮新增。",
      unnamed: "未命名 Node",
      summary: "node: {node} · {count} 个覆盖设备"
    },
    tips: {
      generated: "自动生成：<code>uuid</code>、<code>minor</code>、<code>attr.project-hami.io/minor</code>、<code>pcieBusID</code>",
      preserved: "默认保留：<code>requestPolicy</code>、<code>driverVersion</code>、CUDA 相关字段"
    },
    raw: {
      title: "原始配置",
      description: "这是 <code>data.config.yaml</code> 内容。左侧可视化编辑器会生成它，你也可以直接微调，右侧 ConfigMap 会实时更新。"
    },
    output: {
      title: "最终 ConfigMap",
      description: "可直接 <code>kubectl apply -f</code>。"
    },
    status: {
      generated: "跟随表单自动生成",
      dirty: "原始配置已手动修改"
    },
    placeholders: {
      emptyLabelValue: "留空表示匹配空值标签"
    },
    fields: {
      groupName: "分组名",
      labelKey: "标签 Key",
      labelValue: "标签 Value",
      deviceCount: "设备数量",
      deviceNamePrefix: "设备名前缀",
      minorStart: "起始 minor",
      productName: "GPU 型号",
      memory: "显存",
      cores: "核心数",
      advanced: "高级配置",
      pcieBusStart: "PCIe 起始 Bus(16进制)",
      allowMultipleAllocations: "自动启用 `allowMultipleAllocations`",
      nodeName: "节点名"
    },
    copySuccess: {
      raw: "已复制原始配置",
      configMap: "已复制 ConfigMap"
    }
  }
};

const state = {
  bootstrap: null,
  locale: detectLocale(),
  rawDirty: false,
  generatedRawConfig: "",
  builder: {
    namespace: "default",
    name: "fake-dra-config",
    key: "config.yaml",
    nodeSelectorKey: "",
    nodeSelectorValue: "",
    groups: [],
    nodes: []
  }
};

const elements = {
  namespace: document.getElementById("namespace"),
  name: document.getElementById("name"),
  key: document.getElementById("key"),
  nodeSelectorKey: document.getElementById("node-selector-key"),
  nodeSelectorValue: document.getElementById("node-selector-value"),
  groupsList: document.getElementById("groups-list"),
  nodesList: document.getElementById("nodes-list"),
  addGroup: document.getElementById("add-group"),
  addNode: document.getElementById("add-node"),
  groupTemplate: document.getElementById("group-card-template"),
  nodeTemplate: document.getElementById("node-card-template"),
  rawConfig: document.getElementById("raw-config"),
  configMapOutput: document.getElementById("configmap-output"),
  rawStatus: document.getElementById("raw-status"),
  regenerateRaw: document.getElementById("regenerate-raw"),
  copyRaw: document.getElementById("copy-raw"),
  copyConfigMap: document.getElementById("copy-configmap"),
  resetForm: document.getElementById("reset-form"),
  languageSelect: document.getElementById("language-select")
};

document.addEventListener("DOMContentLoaded", async () => {
  try {
    syncLanguageSelector();
    applyTranslations();
    await loadDefaults();
    bindEvents();
    renderFromForm();
  } catch (error) {
    document.body.innerHTML = `<pre style="padding:24px;color:#b91c1c;">${error.message}</pre>`;
  }
});

async function loadDefaults() {
  const response = await fetch("/api/defaults");
  if (!response.ok) {
    throw new Error(t("errorLoadDefaults"));
  }

  state.bootstrap = await response.json();
  applyBootstrap(state.bootstrap);
}

function applyBootstrap(defaults) {
  state.builder = {
    namespace: defaults.defaultNamespace,
    name: defaults.defaultName,
    key: defaults.defaultKey,
    nodeSelectorKey: defaults.form.nodeSelectorKey,
    nodeSelectorValue: defaults.form.nodeSelectorValue,
    groups: (defaults.form.groups || []).map((group) => withId(group)),
    nodes: (defaults.form.nodes || []).map((node) => withId(node))
  };

  syncStaticInputsFromState();
  renderCollections();
}

function bindEvents() {
  const formInputs = [
    elements.namespace,
    elements.name,
    elements.key,
    elements.nodeSelectorKey,
    elements.nodeSelectorValue
  ];

  formInputs.forEach((element) => {
    element.addEventListener("input", () => {
      syncStaticStateFromInputs();
      renderFromForm();
    });
  });

  elements.languageSelect.addEventListener("change", (event) => {
    setLocale(event.target.value);
  });

  elements.addGroup.addEventListener("click", () => {
    state.builder.groups.push(createDefaultGroup());
    renderCollections();
    renderFromForm();
  });

  elements.addNode.addEventListener("click", () => {
    state.builder.nodes.push(createDefaultNode());
    renderCollections();
    renderFromForm();
  });

  elements.rawConfig.addEventListener("input", () => {
    state.rawDirty = true;
    updateStatus();
    updateConfigMapOutput();
  });

  elements.regenerateRaw.addEventListener("click", () => {
    state.rawDirty = false;
    elements.rawConfig.value = state.generatedRawConfig;
    updateStatus();
    updateConfigMapOutput();
  });

  elements.copyRaw.addEventListener("click", async () => {
    await copyText(elements.rawConfig.value, elements.copyRaw, t("copySuccess.raw"));
  });

  elements.copyConfigMap.addEventListener("click", async () => {
    await copyText(elements.configMapOutput.value, elements.copyConfigMap, t("copySuccess.configMap"));
  });

  elements.resetForm.addEventListener("click", () => {
    if (!state.bootstrap) {
      return;
    }
    state.rawDirty = false;
    applyBootstrap(state.bootstrap);
    renderFromForm();
  });
}

function renderFromForm() {
  syncStaticStateFromInputs();
  state.generatedRawConfig = buildRawConfigFromForm();
  if (!state.rawDirty) {
    elements.rawConfig.value = state.generatedRawConfig;
  }
  updateStatus();
  updateConfigMapOutput();
}

function updateStatus() {
  if (state.rawDirty) {
    elements.rawStatus.textContent = t("status.dirty");
    elements.rawStatus.classList.add("dirty");
    return;
  }
  elements.rawStatus.textContent = t("status.generated");
  elements.rawStatus.classList.remove("dirty");
}

function updateConfigMapOutput() {
  const config = elements.rawConfig.value.trimEnd();
  const result = {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      namespace: elements.namespace.value || "default",
      name: elements.name.value || "fake-dra-config"
    },
    data: {
      [elements.key.value || "config.yaml"]: blockString(config)
    }
  };

  elements.configMapOutput.value = toYAML(result).replace(/!!block\|/g, "|");
}

function buildRawConfigFromForm() {
  const config = {};
  if (state.builder.nodeSelectorKey) {
    config.nodeSelector = {
      matchLabels: {
        [state.builder.nodeSelectorKey]: state.builder.nodeSelectorValue || ""
      }
    };
  }

  const groups = state.builder.groups
    .filter((group) => group.name || group.selectorKey || group.selectorValue)
    .map((group) => ({
      name: group.name || "group",
      selector: {
        matchLabels: {
          [group.selectorKey || "gpu-type"]: group.selectorValue || ""
        }
      },
      devices: buildDevicesFromTemplate(group)
    }));
  if (groups.length > 0) {
    config.groups = groups;
  }

  const nodes = {};
  state.builder.nodes
    .filter((node) => node.nodeName)
    .forEach((node) => {
      nodes[node.nodeName] = {
        devices: buildDevicesFromTemplate(node)
      };
    });
  if (Object.keys(nodes).length > 0) {
    config.nodes = nodes;
  }

  return toYAML(config);
}

function buildDevicesFromTemplate(template) {
  const deviceCount = Math.max(1, parseInt(template.deviceCount || "1", 10));
  const minorStart = parseInt(template.minorStart || "0", 10);
  const pcieBusStart = parseHex(template.pcieBusStart || "61", 0x61);

  return Array.from({ length: deviceCount }, (_, index) => {
    const minor = minorStart + index;
    const busID = generateBusID(pcieBusStart + index);
    const prefix = template.deviceNamePrefix || "gpu";
    const cores = template.cores || "100";
    const memory = template.memory || "80Gi";
    return {
      name: `${prefix}-${minor}`,
      allowMultipleAllocations: Boolean(template.allowMultipleAllocations),
      attributes: {
        architecture: { string: template.architecture || "Ampere" },
        "attr.project-hami.io/minor": { int: minor },
        brand: { string: template.brand || "Nvidia" },
        cudaComputeCapability: { version: template.cudaComputeCapability || "8.0.0" },
        cudaDriverVersion: { version: template.cudaDriverVersion || "12.9.0" },
        driverVersion: { version: template.driverVersion || "575.57.8" },
        minor: { int: minor },
        pcieBusID: { string: busID },
        productName: { string: template.productName || "NVIDIA A100-SXM4-80GB" },
        "resource.kubernetes.io/pcieRoot": { string: template.pcieRoot || "pci0000:5a" },
        type: { string: template.deviceType || "hami-gpu" },
        uuid: { string: generateGPUUUID() }
      },
      capacity: {
        cores: {
          value: cores,
          requestPolicy: {
            default: cores,
            validRange: {
              min: "0",
              max: cores,
              step: "1"
            }
          }
        },
        memory: {
          value: memory,
          requestPolicy: {
            default: memory,
            validRange: {
              min: "1Mi",
              max: memory,
              step: "1Mi"
            }
          }
        }
      }
    };
  });
}

function syncStaticInputsFromState() {
  elements.namespace.value = state.builder.namespace;
  elements.name.value = state.builder.name;
  elements.key.value = state.builder.key;
  elements.nodeSelectorKey.value = state.builder.nodeSelectorKey;
  elements.nodeSelectorValue.value = state.builder.nodeSelectorValue;
}

function syncStaticStateFromInputs() {
  state.builder.namespace = elements.namespace.value;
  state.builder.name = elements.name.value;
  state.builder.key = elements.key.value;
  state.builder.nodeSelectorKey = elements.nodeSelectorKey.value;
  state.builder.nodeSelectorValue = elements.nodeSelectorValue.value;
}

function renderCollections() {
  renderCollectionList({
    type: "group",
    list: state.builder.groups,
    container: elements.groupsList,
    template: elements.groupTemplate
  });
  renderCollectionList({
    type: "node",
    list: state.builder.nodes,
    container: elements.nodesList,
    template: elements.nodeTemplate
  });
}

function renderCollectionList({ type, list, container, template }) {
  container.innerHTML = "";
  list.forEach((item) => {
    const fragment = template.content.cloneNode(true);
    const root = fragment.querySelector(".collection-card");
    root.dataset.id = item.id;
    refreshCardHeader(root, type, item);

    root.querySelectorAll("[data-field]").forEach((field) => {
      const key = field.dataset.field;
      if (field.type === "checkbox") {
        field.checked = Boolean(item[key]);
      } else {
        field.value = item[key] ?? "";
      }

      field.addEventListener(field.type === "checkbox" ? "change" : "input", (event) => {
        const targetItem = list.find((entry) => entry.id === item.id);
        if (!targetItem) {
          return;
        }
        targetItem[key] = field.type === "checkbox" ? event.target.checked : event.target.value;
        refreshCardHeader(root, type, targetItem);
        renderFromForm();
      });
    });

    root.querySelector("[data-action=remove]").addEventListener("click", () => {
      const index = list.findIndex((entry) => entry.id === item.id);
      if (index >= 0) {
        list.splice(index, 1);
        renderCollections();
        renderFromForm();
      }
    });

    container.appendChild(fragment);
  });

  if (list.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.textContent = type === "group" ? t("groups.empty") : t("nodes.empty");
    container.appendChild(empty);
  }
}

function refreshCardHeader(root, type, item) {
  root.querySelector("[data-role=title]").textContent = type === "group" ? (item.name || t("groups.unnamed")) : (item.nodeName || t("nodes.unnamed"));
  root.querySelector("[data-role=subtitle]").textContent = summarizeCollection(type, item);
  applyTranslations(root);
}

function summarizeCollection(type, item) {
  const deviceCount = Math.max(1, parseInt(item.deviceCount || "1", 10));
  if (type === "group") {
    return t("groups.summary", {
      selector: `${item.selectorKey || "gpu-type"}=${item.selectorValue || ""}`,
      count: deviceCount
    });
  }
  return t("nodes.summary", {
    node: item.nodeName || "-",
    count: deviceCount
  });
}

function createDefaultGroup() {
  return withId(cloneObject(state.bootstrap.form.groupTemplate));
}

function createDefaultNode() {
  return withId(cloneObject(state.bootstrap.form.nodeTemplate));
}

function cloneObject(value) {
  return JSON.parse(JSON.stringify(value));
}

function withId(value) {
  return {
    ...value,
    id: crypto.randomUUID()
  };
}

function blockString(value) {
  return `!!block|\n${value}`;
}

function toYAML(value, indent = 0) {
  if (typeof value === "string" && value.startsWith("!!block|\n")) {
    const blockContent = value.slice("!!block|\n".length);
    return `|${blockContent ? "\n" : ""}${indentMultiline(blockContent, indent + 2)}`;
  }

  if (Array.isArray(value)) {
    if (value.length === 0) {
      return "[]";
    }
    return value.map((item) => {
      const rendered = toYAML(item, indent + 2);
      if (isScalar(item)) {
        return `${" ".repeat(indent)}- ${rendered}`;
      }
      const lines = rendered.split("\n");
      return `${" ".repeat(indent)}- ${lines[0].trimStart()}\n${lines.slice(1).join("\n")}`;
    }).join("\n");
  }

  if (value && typeof value === "object") {
    const entries = Object.entries(value).filter(([, entryValue]) => entryValue !== undefined && entryValue !== null);
    if (entries.length === 0) {
      return "{}";
    }
    return entries.map(([key, entryValue]) => {
      const rendered = toYAML(entryValue, indent + 2);
      const prefix = `${" ".repeat(indent)}${formatKey(key)}:`;
      if (isScalar(entryValue) || (typeof entryValue === "string" && rendered.startsWith("|"))) {
        return `${prefix} ${rendered}`;
      }
      return `${prefix}\n${rendered}`;
    }).join("\n");
  }

  return formatScalar(value);
}

function isScalar(value) {
  return value === null || ["string", "number", "boolean"].includes(typeof value);
}

function formatKey(value) {
  return /^[A-Za-z0-9._/-]+$/.test(value) ? value : JSON.stringify(value);
}

function formatScalar(value) {
  if (typeof value === "number") {
    return String(value);
  }
  if (typeof value === "boolean") {
    return value ? "true" : "false";
  }
  if (value === null || value === undefined || value === "") {
    return "\"\"";
  }
  return JSON.stringify(String(value));
}

function indentMultiline(value, indent) {
  return value
    .split("\n")
    .map((line) => `${" ".repeat(indent)}${line}`)
    .join("\n");
}

function generateGPUUUID() {
  const uuid = crypto.randomUUID().toUpperCase();
  return `GPU-${uuid}`;
}

function generateBusID(busNumber) {
  const busHex = busNumber.toString(16).padStart(2, "0");
  return `0000:${busHex}:00.0`;
}

function parseHex(value, fallback) {
  const parsed = parseInt(value, 16);
  return Number.isNaN(parsed) ? fallback : parsed;
}

async function copyText(text, button, successText) {
  const originalText = button.textContent;
  await navigator.clipboard.writeText(text);
  button.textContent = successText;
  setTimeout(() => {
    applyTranslations(button.parentElement || document);
  }, 1200);
}

function detectLocale() {
  const savedLocale = window.localStorage.getItem("fake-dra-confgen-locale");
  if (savedLocale && translations[savedLocale]) {
    return savedLocale;
  }
  return navigator.language && navigator.language.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

function syncLanguageSelector() {
  elements.languageSelect.value = state.locale;
}

function setLocale(locale) {
  if (!translations[locale]) {
    return;
  }
  state.locale = locale;
  window.localStorage.setItem("fake-dra-confgen-locale", locale);
  syncLanguageSelector();
  applyTranslations();
  renderCollections();
  updateStatus();
  updateConfigMapOutput();
}

function applyTranslations(root = document) {
  document.documentElement.lang = state.locale;
  document.title = t("pageTitle");

  root.querySelectorAll("[data-i18n]").forEach((element) => {
    element.textContent = t(element.dataset.i18n);
  });
  root.querySelectorAll("[data-i18n-html]").forEach((element) => {
    element.innerHTML = t(element.dataset.i18nHtml);
  });
  root.querySelectorAll("[data-i18n-placeholder]").forEach((element) => {
    element.placeholder = t(element.dataset.i18nPlaceholder);
  });
}

function t(key, params = {}) {
  const value = key.split(".").reduce((result, segment) => (result ? result[segment] : undefined), translations[state.locale]) ??
    key.split(".").reduce((result, segment) => (result ? result[segment] : undefined), translations.en) ??
    key;

  if (typeof value !== "string") {
    return String(value);
  }
  return value.replace(/\{(\w+)\}/g, (_, token) => params[token] ?? `{${token}}`);
}
