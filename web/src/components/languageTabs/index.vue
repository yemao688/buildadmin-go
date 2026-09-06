<template>
    <div class="lang" v-if="langNames.length">
        <div class="lang-tabs">
            <el-tabs v-model="tabValue" addable class="tabstopwidth" @tab-change="onChangeTab">
                <template #add-icon>
                    <el-button type="primary" class="leftmargin" @click="translate()">翻译</el-button>
                </template>
                <el-tab-pane v-for="(item, index) in langNames" :key="index" :label="item.remark" :name="index"> </el-tab-pane>
            </el-tabs>
        </div>

        <div class="wraper">
            <div class="wraper-item">
                <template v-if="props.type == 'input' && !props.editor">
                    <el-input v-model="langNames[tabValue].value" placeholder="请输入" />
                </template>
                <template v-else>
                    <EditorComp v-model="langNames[tabValue].value"></EditorComp>
                </template>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useLanguageTabs } from '/@/stores/languageTab'
import type { langItem } from '/@/stores/interface/index'
import { cloneDeep } from 'lodash-es'
import EditorComp from '../baInput/components/editor.vue'
import { getMultTranslations } from '/@/api/backend/country/language'
import { ElMessage, ElLoading } from 'element-plus'

const emits = defineEmits(['update:modelValue', 'translate'])

interface Props {
    // 多语言内容值（withDefaults 提供默认空数组），父组件可能初始不传
    modelValue?: langItem[]
    showDefault?: boolean
    type?: 'input' | 'editor'
    // 富文本开关：baInput 字段 type 被分发机制占用（'languageTabs'），
    // 编辑器形态用 editor: true 声明（等价 type: 'editor'）
    editor?: boolean
    defaultValue?: string
}

const props = withDefaults(defineProps<Props>(), {
    modelValue: () => [],
    defaultValue: '',
    showDefault: true,
    type: 'input',
    editor: false,
})

const onValueUpdate = () => {
    emits('update:modelValue', langNames.value)
}
const onSuccess = () => {
    emits('translate', enValue.value)
}
const langStore = useLanguageTabs()
const enValue = ref()
const tabValue = ref(0)
const onChangeTab = () => {}
const langList = ref<langItem[]>([])
const data = ref({
    lan_value: '',
    lan: '',
})
const langNames = ref<langItem[]>([])

watch(
    langNames,
    () => {
        onValueUpdate()
    },
    {
        deep: true,
    }
)

watch(
    () => props.modelValue,
    (nVal) => {
        if (!nVal || nVal.length == 0) {
            clear()
        }
    }
)

const fillLang = () => {
    let len = props?.modelValue?.length ?? 0

    let arr: langItem[] = []
    // 注意：langList 直接引用 store 成员会污染持久化语言列表——
    // fillLang 的写入必须作用在副本上（onMounted 已 cloneDeep）
    langList.value.forEach((item: langItem) => {
        // 标记, 若未匹配到该语言, 置空
        let flag = false
        let obj = item
        for (let i = 0; i < len; i++) {
            if (item.lan == props.modelValue?.[i]?.lan) {
                item.value = props.modelValue?.[i]?.value ?? ''
                arr.push(obj)
                flag = true
                break
            }
        }
        if (!flag) {
            item.value = ''
            arr.push(obj)
        }
    })

    langNames.value = cloneDeep(arr)

    if (!props.showDefault) {
        ;[, ...langNames.value] = langNames.value
    }
}

const clear = () => {
    langNames.value.forEach((item: langItem) => {
        item.value = ''
    })
    tabValue.value = 0
}
const translate = () => {
    let defaultValue = props.defaultValue

    if (defaultValue) {
        // 源语言取默认语言（总语言列表首项，weigh DESC），不能硬编码 en：
        // 部署的默认语言可能是 zh-cn（seed weigh 2 > en 1）
        data.value.lan = langStore.state.langList[0]?.lan || 'en'
        data.value.lan_value = defaultValue
    } else {
        data.value.lan = langNames.value[tabValue.value].lan
        data.value.lan_value = langNames.value[tabValue.value].value
    }
    const loading = ElLoading.service({
        lock: true,
        text: '翻译中，请等待....',
        background: 'rgba(0, 0, 0, 0.7)',
    })
    getMultTranslations(data.value)
        .then((res) => {
            // 遍历 lanValue 数组
            res.data.forEach((item: langItem) => {
                // 在 lanValueArray 中找到与当前对象 lan 属性相同的对象
                const matchedObject = langNames.value.find((obj) => obj.lan === item.lan)

                if (matchedObject) {
                    matchedObject.value = item.value
                }
            })
            loading.close()

            ElMessage.success('翻译成功')
            if (!defaultValue) {
                // 找到 lan 等于 "en" 的对象
                const enObject = res.data.find((item: langItem) => item.lan === 'en')

                // 获取 enObject 对象的 value 属性
                enValue.value = enObject ? enObject.value : ''

                onSuccess()
            }
        })
        .catch(() => {
            loading.close()
        })
}
defineExpose({
    clear,
})

onMounted(() => {
    // cloneDeep：fillLang 会在 langList 成员上写 value，直接引用 store 会
    // 污染持久化的语言列表（其它表单会读到本表单的内容），必须副本化
    langList.value = cloneDeep(langStore.state.langList)

    fillLang()
})
</script>

<style lang="scss" scoped>
.lang {
    width: 100%;
}
.lang-tabs {
    width: 90%;
}
.leftmargin {
    margin-left: 40px;
}

.wraper {
    display: flex;
    align-items: center;
    justify-content: flex-start;
}
.wraper-item {
    flex: 1 1 auto;
}
</style>