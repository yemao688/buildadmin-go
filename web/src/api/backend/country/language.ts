import createAxios from '/@/utils/axios'

export function getMultTranslations(data: any) {
    return createAxios({
        url: '/admin/country.Language/getMultTranslations',
        method: 'post',
        data,
    })
}