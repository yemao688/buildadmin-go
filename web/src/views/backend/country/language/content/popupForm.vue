<template>
    <!-- 对话框表单 -->
    <!-- 建议使用 Prettier 格式化代码 -->
    <!-- el-form 内可以混用 el-form-item、FormItem、ba-input 等输入组件 -->
    <el-dialog
        class="ba-operate-dialog"
        :close-on-click-modal="false"
        :model-value="['Add', 'Edit'].includes(baTable.form.operate!)"
        @close="baTable.toggleForm"
        width="560px"
    >
        <template #header>
            <div class="title" v-drag="['.ba-operate-dialog', '.el-dialog__header']" v-zoom="'.ba-operate-dialog'">
                {{ baTable.form.operate ? t(baTable.form.operate) : '' }}
            </div>
        </template>
        <el-scrollbar v-loading="baTable.form.loading" class="ba-table-form-scrollbar">
            <div
                class="ba-operate-form"
                :class="'ba-' + baTable.form.operate + '-form'"
                :style="config.layout.shrink ? '' : 'width: calc(100% - ' + baTable.form.labelWidth! / 2 + 'px)'"
            >
                <el-form
                    v-if="!baTable.form.loading"
                    ref="formRef"
                    @submit.prevent=""
                    @keyup.enter="baTable.onSubmit(formRef)"
                    :model="baTable.form.items"
                    :label-position="config.layout.shrink ? 'top' : 'right'"
                    :label-width="baTable.form.labelWidth + 'px'"
                    :rules="rules"
                >
                    <FormItem
                        :label="t('country.language.content.lan')"
                        type="string"
                        v-model="baTable.form.items!.lan"
                        prop="lan"
                        :placeholder="
                            t('Please input field', {
                                field: t('country.language.content.lan'),
                            })
                        "
                    />
                    <FormItem
                        :label="t('country.language.content.group')"
                        type="string"
                        v-model="baTable.form.items!.group"
                        prop="group"
                        :placeholder="
                            t('Please input field', {
                                field: t('country.language.content.group'),
                            })
                        "
                    />
                    <FormItem
                        :label="t('country.language.content.key')"
                        type="string"
                        v-model="baTable.form.items!.key"
                        prop="key"
                        :placeholder="
                            t('Please input field', {
                                field: t('country.language.content.key'),
                            })
                        "
                    />
                    <FormItem
                        :label="t('country.language.content.type')"
                        type="radio"
                        :model-value="String(baTable.form.items!.type ?? 0)"
                        prop="type"
                        :input-attr="{
                            border: true,
                            content: {
                                '0': t('country.language.content.type 0'),
                                '1': t('country.language.content.type 1'),
                                '2': t('country.language.content.type 2'),
                            },
                        }"
                        @update:model-value="onTypeChange"
                    />
                    <FormItem
                        v-if="contentType === 0"
                        :label="t('country.language.content.value')"
                        type="textarea"
                        v-model="baTable.form.items!.value"
                        prop="value"
                        :input-attr="{
                            rows: 4,
                            autosize: { minRows: 3, maxRows: 8 },
                        }"
                        @keyup.enter.stop=""
                        @keyup.ctrl.enter="baTable.onSubmit(formRef)"
                        :placeholder="
                            t('Please input field', {
                                field: t('country.language.content.value'),
                            })
                        "
                    />
                    <FormItem
                        v-else-if="contentType === 1"
                        :label="t('country.language.content.value')"
                        type="editor"
                        v-model="baTable.form.items!.value"
                        prop="value"
                        @keyup.enter.stop=""
                        :placeholder="
                            t('Please input field', {
                                field: t('country.language.content.value'),
                            })
                        "
                    />
                    <FormItem
                        v-else
                        :label="t('country.language.content.value')"
                        type="image"
                        v-model="baTable.form.items!.value"
                        prop="value"
                    />
                </el-form>
            </div>
        </el-scrollbar>
        <template #footer>
            <div :style="'width: calc(100% - ' + baTable.form.labelWidth! / 1.8 + 'px)'">
                <el-button @click="baTable.toggleForm()">{{ t('Cancel') }}</el-button>
                <el-button v-blur :loading="baTable.form.submitLoading" @click="baTable.onSubmit(formRef)" type="primary">
                    {{ baTable.form.operateIds && baTable.form.operateIds.length > 1 ? t('Save and edit next item') : t('Save') }}
                </el-button>
            </div>
        </template>
    </el-dialog>
</template>

<script setup lang="ts">
import type { FormInstance, FormItemRule } from 'element-plus'
import { computed, inject, reactive, ref, type Ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import FormItem from '/@/components/formItem/index.vue'
import { useConfig } from '/@/stores/config'
import type baTableClass from '/@/utils/baTable'
import { buildValidatorData } from '/@/utils/validate'

const config = useConfig()
const formRef = ref<FormInstance>()
const baTable = inject('baTable') as baTableClass
const selectedLan = inject('countryContentLanguage', ref('')) as Ref<string>

const { t } = useI18n()

const rules: Partial<Record<string, FormItemRule[]>> = reactive({})
const contentType = computed(() => Number(baTable.form.items!.type ?? 0))

const onTypeChange = (value: string | number) => {
    baTable.form.items!.type = Number(value)
}

watch(
    [selectedLan, () => baTable.form.operate],
    ([lan]) => {
        if (baTable.form.operate === 'Add' && lan) baTable.form.items!.lan = lan
    },
    { immediate: true }
)
</script>

<style scoped lang="scss">
:deep(.ba-input-item-radio .el-radio) {
    margin-right: 8px;
    margin-bottom: 4px;
}

@media screen and (max-width: 600px) {
    :global(.ba-operate-dialog) {
        width: calc(100% - 24px) !important;
    }
}
</style>
