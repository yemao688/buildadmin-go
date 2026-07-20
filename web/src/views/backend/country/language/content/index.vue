<template>
    <div class="default-main ba-table-box country-content-page">
        <el-alert
            class="ba-table-alert"
            v-if="baTable.table.remark"
            :title="baTable.table.remark"
            type="info"
            show-icon
        />

        <div
            class="language-filter"
            :aria-label="t('country.language.content.all languages')"
        >
            <el-tabs v-model="selectedLan" @tab-change="onLanguageChange">
                <el-tab-pane
                    :label="t('country.language.content.all languages')"
                    name=""
                />
                <el-tab-pane
                    v-for="language in languages"
                    :key="language.lan"
                    :label="language.name || language.remark || language.lan"
                    :name="language.lan"
                />
            </el-tabs>
        </div>

        <!-- 表格顶部菜单 -->
        <!-- 自定义按钮请使用插槽，甚至公共搜索也可以使用具名插槽渲染，参见文档 -->
        <TableHeader
            :buttons="[
                'refresh',
                'add',
                'edit',
                'delete',
                'comSearch',
                'quickSearch',
                'columnDisplay',
            ]"
            :quick-search-placeholder="
                t('Quick search placeholder', {
                    fields: t('country.language.content.quick Search Fields'),
                })
            "
        ></TableHeader>

        <!-- 表格 -->
        <!-- 表格列有多种自定义渲染方式，比如自定义组件、具名插槽等，参见文档 -->
        <!-- 要使用 el-table 组件原有的属性，直接加在 Table 标签上即可 -->
        <Table ref="tableRef"></Table>

        <!-- 表单 -->
        <PopupForm />
    </div>
</template>

<script setup lang="ts">
import { onMounted, provide, ref } from "vue";
import { useI18n } from "vue-i18n";
import PopupForm from "./popupForm.vue";
import { baTableApi } from "/@/api/common";
import { defaultOptButtons } from "/@/components/table";
import TableHeader from "/@/components/table/header/index.vue";
import Table from "/@/components/table/index.vue";
import baTableClass from "/@/utils/baTable";

defineOptions({
    name: "country/language/content",
});

const { t } = useI18n();
const tableRef = ref();
const selectedLan = ref("");
const languages = ref<{ lan: string; name?: string; remark?: string }[]>([]);
const optButtons: OptButton[] = defaultOptButtons(["edit", "delete"]);

const withLanguageFilter = (search: ComSearchData[]) => {
    const filteredSearch = search.filter((item) => item.field !== "lan");
    if (selectedLan.value) {
        filteredSearch.push({
            field: "lan",
            val: selectedLan.value,
            operator: "eq",
            render: "string",
        });
    }
    return filteredSearch;
};

/**
 * baTable 内包含了表格的所有数据且数据具备响应性，然后通过 provide 注入给了后代组件
 */
const baTable = new baTableClass(
    new baTableApi("/admin/countryLanguageContent/"),
    {
        pk: "id",
        column: [
            { type: "selection", align: "center", operator: false },
            {
                label: t("country.language.content.id"),
                prop: "id",
                align: "center",
            },
            {
                label: t("country.language.content.lan"),
                prop: "lan",
                align: "center",
            },
            {
                label: t("country.language.content.group"),
                prop: "group",
                align: "center",
            },
            {
                label: t("country.language.content.key"),
                prop: "key",
                align: "center",
            },
            {
                label: t("country.language.content.type"),
                prop: "type",
                align: "center",
                replaceValue: {
                    "0": t("country.language.content.type 0"),
                    "1": t("country.language.content.type 1"),
                    "2": t("country.language.content.type 2"),
                },
            },
            {
                label: t("country.language.content.value"),
                prop: "value",
                align: "center",
            },
            {
                label: t("Operate"),
                align: "center",
                width: 100,
                render: "buttons",
                buttons: optButtons,
                operator: false,
            },
        ],
        dblClickNotEditColumn: [undefined],
    },
    {
        defaultItems: { type: 0 },
    },
);

provide("baTable", baTable);
provide("countryContentLanguage", selectedLan);

const onLanguageChange = (lan: string | number) => {
    selectedLan.value = String(lan);
    baTable.setFilterSearchData(
        withLanguageFilter(baTable.table.filter?.search || []),
        "cover",
    );
    baTable.onTableHeaderAction("refresh", {
        event: "language-change",
        lan: selectedLan.value,
    });
};

baTable.before.onTableAction = ({ event }) => {
    if (event === "com-search") {
        baTable.setFilterSearchData(
            withLanguageFilter(baTable.getComSearchData()),
            "cover",
        );
        baTable.onTableHeaderAction("refresh", {
            event: "com-search",
            data: baTable.table.filter!.search,
        });
        return false;
    }
};

const getLanguages = () => {
    return new baTableApi("/admin/countryLanguage/")
        .index({ limit: 1000 })
        .then((res) => {
            languages.value = (res.data.list || []).map((item: anyObj) => ({
                lan: String(item.lan),
                name: item.name,
                remark: item.remark,
            }));
        });
};

onMounted(() => {
    baTable.table.ref = tableRef.value;
    baTable.mount();
    getLanguages();
    baTable.getIndex()?.then(() => {
        baTable.initSort();
        baTable.dragSort();
    });
});
</script>

<style scoped lang="scss">
.language-filter {
    margin: -4px 0 12px;
    border-bottom: 1px solid var(--el-border-color-lighter);

    :deep(.el-tabs__header) {
        margin: 0;
    }
}

@media screen and (max-width: 768px) {
    .language-filter {
        margin-bottom: 8px;

        :deep(.el-tabs__nav-wrap) {
            overflow-x: auto;
        }

        :deep(.el-tabs__nav) {
            min-width: max-content;
        }
    }
}
</style>
