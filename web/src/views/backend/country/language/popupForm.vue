<template>
    <!-- 对话框表单 -->
    <!-- 建议使用 Prettier 格式化代码 -->
    <!-- el-form 内可以混用 el-form-item、FormItem、ba-input 等输入组件 -->
    <el-dialog
        class="ba-operate-dialog"
        :close-on-click-modal="false"
        :model-value="['Add', 'Edit'].includes(baTable.form.operate!)"
        @close="baTable.toggleForm"
        width="520px"
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
                        :label="t('country.language.lan')"
                        type="string"
                        v-model="baTable.form.items!.lan"
                        prop="lan"
                        :placeholder="
                            t('Please input field', {
                                field: t('country.language.lan'),
                            })
                        "
                    />
                    <FormItem
                        :label="t('country.language.name')"
                        type="string"
                        v-model="baTable.form.items!.name"
                        prop="name"
                        :placeholder="
                            t('Please input field', {
                                field: t('country.language.name'),
                            })
                        "
                    />
                    <FormItem
                        :label="t('country.language.remark')"
                        type="string"
                        v-model="baTable.form.items!.remark"
                        prop="remark"
                        :placeholder="
                            t('Please input field', {
                                field: t('country.language.remark'),
                            })
                        "
                    />
                    <FormItem
                        :label="t('country.language.status')"
                        type="number"
                        v-model.number="baTable.form.items!.status"
                        prop="status"
                        :data="{
                            content: {
                                '1': t('country.language.status 1'),
                                '0': t('country.language.status 0'),
                            },
                        }"
                        :placeholder="
                            t('Please input field', {
                                field: t('country.language.status'),
                            })
                        "
                    />
                    <FormItem
                        :label="t('country.language.weigh')"
                        type="number"
                        v-model.number="baTable.form.items!.weigh"
                        prop="weigh"
                        :input-attr="{ step: 1 }"
                        :placeholder="
                            t('Please input field', {
                                field: t('country.language.weigh'),
                            })
                        "
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
import { inject, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import FormItem from '/@/components/formItem/index.vue'
import { useConfig } from '/@/stores/config'
import type baTableClass from '/@/utils/baTable'
import { buildValidatorData } from '/@/utils/validate'

const config = useConfig()
const formRef = ref<FormInstance>()
const baTable = inject('baTable') as baTableClass

const { t } = useI18n()

const rules: Partial<Record<string, FormItemRule[]>> = reactive({})
</script>

<style scoped lang="scss">
@media screen and (max-width: 600px) {
    :global(.ba-operate-dialog) {
        width: calc(100% - 24px) !important;
    }
}
</style>
