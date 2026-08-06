<template>
    <div class="default-main ba-table-box">
        <el-alert class="ba-table-alert" v-if="baTable.table.remark" :title="baTable.table.remark" type="info" show-icon />

        <!-- 表格顶部菜单 -->
        <TableHeader
            :buttons="['refresh', 'add', 'edit', 'delete', 'unfold', 'comSearch', 'quickSearch', 'columnDisplay']"
            :quick-search-placeholder="t('Quick search placeholder', { fields: t('auth.admin.username') + '/' + t('auth.admin.nickname') })"
        />

        <!-- 表格 -->
        <!-- 要使用`el-table`组件原有的属性，直接加在Table标签上即可 -->
        <Table ref="tableRef" :pagination="false" :tree-props="{ children: 'children' }" />

        <!-- 表单 -->
        <PopupForm />
    </div>
</template>

<script setup lang="ts">
import { onMounted, provide, useTemplateRef } from 'vue'
import baTableClass from '/@/utils/baTable'
import PopupForm from './popupForm.vue'
import Table from '/@/components/table/index.vue'
import TableHeader from '/@/components/table/header/index.vue'
import { defaultOptButtons } from '/@/components/table'
import { baTableApi } from '/@/api/common'
import { useAdminInfo } from '/@/stores/adminInfo'
import { useI18n } from 'vue-i18n'

defineOptions({
    name: 'auth/admin',
})

const { t } = useI18n()
const adminInfo = useAdminInfo()
const tableRef = useTemplateRef('tableRef')

const optButtons = defaultOptButtons(['edit', 'delete'])
optButtons[1].display = (row) => {
    return row.id != adminInfo.id
}

const baTable = new baTableClass(
    new baTableApi('/admin/auth.Admin/'),
    {
        filter: { isTree: 1 },
        expandAll: true,
        column: [
            { label: t('Id'), prop: 'id', align: 'center', operator: '=', operatorPlaceholder: t('Id'), minWidth: 90 },
            { label: t('auth.admin.username'), prop: 'username', align: 'center', operator: 'LIKE', operatorPlaceholder: t('Fuzzy query') },
            { label: t('auth.admin.nickname'), prop: 'nickname', align: 'center', operator: 'LIKE', operatorPlaceholder: t('Fuzzy query') },
            { label: t('auth.admin.Parent agent'), prop: 'parent.username', align: 'center', operator: false, formatter: (row: anyObj) => row.parent?.username || '-' },
            { label: t('auth.admin.group'), prop: 'group_name_arr', align: 'center', operator: false, render: 'tags', minWidth: 80 },
            { label: t('auth.admin.Invite code'), prop: 'invite_code', align: 'center', operator: false, width: 130 },
            {
                label: t('auth.admin.Last login'),
                prop: 'last_login_time',
                align: 'center',
                render: 'datetime',
                sortable: 'custom',
                operator: 'RANGE',
                width: 160,
            },
            { label: t('Create time'), prop: 'create_time', align: 'center', render: 'datetime', sortable: 'custom', operator: 'RANGE', width: 160 },
            {
                label: t('State'),
                prop: 'status',
                align: 'center',
                render: 'tag',
                custom: { disable: 'danger', enable: 'success' },
                replaceValue: { disable: t('Disable'), enable: t('Enable') },
            },
            {
                label: t('Operate'),
                align: 'center',
                width: '100',
                render: 'buttons',
                buttons: optButtons,
                operator: false,
                fixed: 'right',
            },
        ],
        dblClickNotEditColumn: [undefined, 'status'],
    },
    {
        defaultItems: {
            status: 'enable',
        },
    }
)

provide('baTable', baTable)

onMounted(() => {
    baTable.table.ref = tableRef.value
    baTable.mount()
    baTable.getData()
})
</script>

<style scoped lang="scss"></style>
