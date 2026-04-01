"use strict";

const state = {
  bootstrap: null,
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
  resetForm: document.getElementById("reset-form")
};

document.addEventListener("DOMContentLoaded", async () => {
  try {
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
    throw new Error("加载默认配置失败");
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
    await copyText(elements.rawConfig.value, elements.copyRaw, "已复制原始配置");
  });

  elements.copyConfigMap.addEventListener("click", async () => {
    await copyText(elements.configMapOutput.value, elements.copyConfigMap, "已复制 ConfigMap");
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
    elements.rawStatus.textContent = "原始配置已手动修改";
    elements.rawStatus.classList.add("dirty");
    return;
  }
  elements.rawStatus.textContent = "跟随表单自动生成";
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
    empty.textContent = type === "group" ? "还没有 Group，点击右上角新增。" : "还没有 Node 覆盖，点击右上角新增。";
    container.appendChild(empty);
  }
}

function refreshCardHeader(root, type, item) {
  root.querySelector("[data-role=title]").textContent = type === "group" ? (item.name || "未命名 Group") : (item.nodeName || "未命名 Node");
  root.querySelector("[data-role=subtitle]").textContent = summarizeCollection(type, item);
}

function summarizeCollection(type, item) {
  const deviceCount = Math.max(1, parseInt(item.deviceCount || "1", 10));
  if (type === "group") {
    return `selector: ${item.selectorKey || "gpu-type"}=${item.selectorValue || ""} · ${deviceCount} 个设备`;
  }
  return `node: ${item.nodeName || "-"} · ${deviceCount} 个覆盖设备`;
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
    button.textContent = originalText;
  }, 1200);
}
