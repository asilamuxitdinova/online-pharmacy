import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { session } from './api'
import type { Lang } from './types'

const en = {
  'nav.shop': 'Shop',
  'nav.account': 'My pharmacy',
  'nav.staff': 'Pharmacist desk',
  'nav.admin': 'Admin',
  'nav.signin': 'Sign in',
  'nav.signout': 'Sign out',
  'hero.policies': 'How we work',
  'rule.cities': 'Cities we deliver to',
  'rule.sameDay': 'Order before this time for same-day delivery in Tashkent',
  'footer.tagline': 'Online pharmacy delivering medicines and vitamins across Uzbekistan.',
  'footer.help': 'Help',
  'footer.delivery': 'Delivery',
  'footer.refunds': 'Refunds',
  'footer.prescriptions': 'Prescriptions',
  'footer.payments': 'Payments',
  'footer.faq': 'Questions',
  'footer.developers': 'Developers',
  'footer.apiDocs': 'API documentation',
  'nav.cart': 'Basket',
  'hero.kicker': 'Online pharmacy · delivery across Uzbekistan',
  'hero.title': 'Medicine, delivered across Uzbekistan.',
  'hero.sub': 'Over a thousand medicines and vitamins, with prescriptions checked by our pharmacists. Pay by Uzcard, Humo, Visa or Mastercard and get it the same day in Tashkent.',
  'hero.cta': 'Browse the shelves',
  'home.categories': 'Shelves',
  'home.featured': 'Freshly stocked',
  'home.rules': 'House rules, in numbers',
  'rule.delivery': 'Free courier delivery from',
  'rule.refund': 'Refunds approved instantly up to',
  'shop.search': 'Search in any language: ibuprofen, парацетамол, vitamin…',
  'shop.all': 'All shelves',
  'shop.rxOnly': 'Prescription only',
  'shop.inStock': 'In stock',
  'shop.sort': 'Sort',
  'shop.results': 'results',
  'shop.empty': 'Nothing on the shelves matches that.',
  'shop.emptyHint': 'Try another word, or another language — names are searchable in English, Russian and Uzbek.',
  'product.add': 'Add to basket',
  'product.added': 'In basket',
  'product.rx': 'Prescription',
  'product.out': 'Sold out',
  'product.stockCheck': 'Branch availability',
  'product.stockCheckHint': 'See how many packs each of our branches has right now.',
  'product.left': 'left online',
  'cart.title': 'Your basket',
  'cart.empty': 'The basket is empty. Somewhere a vitamin is waiting.',
  'cart.checkout': 'Go to checkout',
  'cart.subtotal': 'Subtotal',
  'checkout.title': 'Checkout',
  'checkout.delivery': 'Delivery',
  'checkout.courier': 'Courier',
  'checkout.pickup': 'Pickup at a branch',
  'checkout.address': 'Address',
  'checkout.prescription': 'Prescription',
  'checkout.place': 'Place order',
  'checkout.pay': 'Pay',
  'checkout.payWith': 'Pay with',
  'checkout.savedCard': 'Saved card',
  'checkout.newCard': 'New card',
  'checkout.confirm': 'I confirm this payment of',
  'checkout.paid': 'Paid. Thank you!',
  'account.orders': 'Orders',
  'account.prescriptions': 'Prescriptions',
  'account.cards': 'Cards',
  'account.addresses': 'Addresses',
  'account.keys': 'API keys',
  'account.profile': 'Profile',
  'order.refund': 'Ask for a refund',
  'order.cancel': 'Cancel order',
  'order.invoice': 'Invoice',
  'order.payNow': 'Pay now',
  'common.loading': 'Loading…',
  'common.more': 'Load more',
  'common.save': 'Save',
  'common.delete': 'Delete',
  'common.cancel': 'Cancel',
  'common.retry': 'Try again',
  'common.close': 'Close',
  'status.pending_payment': 'Waiting for payment',
  'status.paid': 'Paid',
  'status.cancelled': 'Cancelled',
  'status.partially_refunded': 'Partly refunded',
  'status.refunded': 'Refunded',
  'status.pending_review': 'Pharmacist reviewing',
  'status.approved': 'Approved',
  'status.rejected': 'Rejected',
}

type Key = keyof typeof en
type Dict = Record<Key, string>

const ru: Dict = {
  'nav.shop': 'Каталог',
  'nav.account': 'Моя аптека',
  'nav.staff': 'Стол фармацевта',
  'nav.admin': 'Админ',
  'nav.signin': 'Войти',
  'nav.signout': 'Выйти',
  'hero.policies': 'Как мы работаем',
  'rule.cities': 'Городов с доставкой',
  'rule.sameDay': 'Закажите до этого времени — доставим по Ташкенту сегодня',
  'footer.tagline': 'Онлайн-аптека с доставкой лекарств и витаминов по всему Узбекистану.',
  'footer.help': 'Помощь',
  'footer.delivery': 'Доставка',
  'footer.refunds': 'Возвраты',
  'footer.prescriptions': 'Рецепты',
  'footer.payments': 'Оплата',
  'footer.faq': 'Вопросы',
  'footer.developers': 'Разработчикам',
  'footer.apiDocs': 'Документация API',
  'nav.cart': 'Корзина',
  'hero.kicker': 'Онлайн-аптека · доставка по Узбекистану',
  'hero.title': 'Лекарства с доставкой по всему Узбекистану.',
  'hero.sub': 'Более тысячи лекарств и витаминов, рецепты проверяют наши фармацевты. Оплата картами Uzcard, Humo, Visa и Mastercard, доставка по Ташкенту в тот же день.',
  'hero.cta': 'К полкам',
  'home.categories': 'Полки',
  'home.featured': 'Свежие поступления',
  'home.rules': 'Правила в цифрах',
  'rule.delivery': 'Бесплатная доставка курьером от',
  'rule.refund': 'Мгновенный возврат до',
  'shop.search': 'Ищите на любом языке: ибупрофен, paracetamol, витамин…',
  'shop.all': 'Все полки',
  'shop.rxOnly': 'Только по рецепту',
  'shop.inStock': 'В наличии',
  'shop.sort': 'Сортировка',
  'shop.results': 'результатов',
  'shop.empty': 'На полках ничего такого нет.',
  'shop.emptyHint': 'Попробуйте другое слово или другой язык — названия ищутся на английском, русском и узбекском.',
  'product.add': 'В корзину',
  'product.added': 'В корзине',
  'product.rx': 'По рецепту',
  'product.out': 'Нет в наличии',
  'product.stockCheck': 'Наличие в филиалах',
  'product.stockCheckHint': 'Узнайте, сколько упаковок сейчас есть в каждом филиале.',
  'product.left': 'осталось онлайн',
  'cart.title': 'Ваша корзина',
  'cart.empty': 'Корзина пуста. Где-то ждёт витамин.',
  'cart.checkout': 'Оформить заказ',
  'cart.subtotal': 'Сумма',
  'checkout.title': 'Оформление',
  'checkout.delivery': 'Доставка',
  'checkout.courier': 'Курьер',
  'checkout.pickup': 'Самовывоз из филиала',
  'checkout.address': 'Адрес',
  'checkout.prescription': 'Рецепт',
  'checkout.place': 'Оформить заказ',
  'checkout.pay': 'Оплатить',
  'checkout.payWith': 'Оплата',
  'checkout.savedCard': 'Сохранённая карта',
  'checkout.newCard': 'Новая карта',
  'checkout.confirm': 'Подтверждаю оплату',
  'checkout.paid': 'Оплачено. Спасибо!',
  'account.orders': 'Заказы',
  'account.prescriptions': 'Рецепты',
  'account.cards': 'Карты',
  'account.addresses': 'Адреса',
  'account.keys': 'API-ключи',
  'account.profile': 'Профиль',
  'order.refund': 'Запросить возврат',
  'order.cancel': 'Отменить заказ',
  'order.invoice': 'Счёт',
  'order.payNow': 'Оплатить',
  'common.loading': 'Загрузка…',
  'common.more': 'Показать ещё',
  'common.save': 'Сохранить',
  'common.delete': 'Удалить',
  'common.cancel': 'Отмена',
  'common.retry': 'Повторить',
  'common.close': 'Закрыть',
  'status.pending_payment': 'Ждёт оплаты',
  'status.paid': 'Оплачен',
  'status.cancelled': 'Отменён',
  'status.partially_refunded': 'Частичный возврат',
  'status.refunded': 'Возвращён',
  'status.pending_review': 'Проверяет фармацевт',
  'status.approved': 'Одобрен',
  'status.rejected': 'Отклонён',
}

const uz: Dict = {
  'nav.shop': 'Katalog',
  'nav.account': 'Mening dorixonam',
  'nav.staff': 'Farmatsevt stoli',
  'nav.admin': 'Admin',
  'nav.signin': 'Kirish',
  'nav.signout': 'Chiqish',
  'hero.policies': 'Qanday ishlaymiz',
  'rule.cities': 'Yetkazib beradigan shaharlar',
  'rule.sameDay': 'Shu vaqtgacha buyurtma bering — Toshkentda bugun yetkazamiz',
  'footer.tagline': "O'zbekiston bo'ylab dori va vitaminlarni yetkazib beruvchi onlayn dorixona.",
  'footer.help': 'Yordam',
  'footer.delivery': 'Yetkazish',
  'footer.refunds': 'Qaytarish',
  'footer.prescriptions': 'Retseptlar',
  'footer.payments': "To'lov",
  'footer.faq': 'Savollar',
  'footer.developers': 'Dasturchilar uchun',
  'footer.apiDocs': 'API hujjatlari',
  'nav.cart': 'Savat',
  'hero.kicker': "Onlayn dorixona · O'zbekiston bo'ylab yetkazish",
  'hero.title': "Dorilar butun O'zbekiston bo'ylab yetkaziladi.",
  'hero.sub': "Mingdan ortiq dori va vitaminlar, retseptlarni farmatsevtlarimiz tekshiradi. Uzcard, Humo, Visa yoki Mastercard bilan to'lang va Toshkentda shu kuniyoq oling.",
  'hero.cta': 'Javonlarga',
  'home.categories': 'Javonlar',
  'home.featured': 'Yangi kelganlar',
  'home.rules': 'Qoidalar raqamlarda',
  'rule.delivery': 'Bepul kuryer yetkazish',
  'rule.refund': 'Darhol qaytarish',
  'shop.search': 'Istalgan tilda qidiring: ibuprofen, парацетамол, vitamin…',
  'shop.all': 'Barcha javonlar',
  'shop.rxOnly': "Faqat retsept bo'yicha",
  'shop.inStock': 'Mavjud',
  'shop.sort': 'Saralash',
  'shop.results': 'natija',
  'shop.empty': "Javonlarda bunday narsa yo'q.",
  'shop.emptyHint': "Boshqa so'z yoki boshqa tilni sinab ko'ring — nomlar ingliz, rus va o'zbek tillarida qidiriladi.",
  'product.add': "Savatga qo'shish",
  'product.added': 'Savatda',
  'product.rx': "Retsept bo'yicha",
  'product.out': 'Tugagan',
  'product.stockCheck': 'Filiallardagi mavjudlik',
  'product.stockCheckHint': 'Har bir filialda hozir nechta qadoq borligini bilib oling.',
  'product.left': 'onlayn qoldi',
  'cart.title': 'Savatingiz',
  'cart.empty': "Savat bo'sh. Qayerdadir vitamin kutmoqda.",
  'cart.checkout': 'Rasmiylashtirish',
  'cart.subtotal': 'Jami',
  'checkout.title': 'Rasmiylashtirish',
  'checkout.delivery': 'Yetkazish',
  'checkout.courier': 'Kuryer',
  'checkout.pickup': 'Filialdan olib ketish',
  'checkout.address': 'Manzil',
  'checkout.prescription': 'Retsept',
  'checkout.place': 'Buyurtma berish',
  'checkout.pay': "To'lash",
  'checkout.payWith': "To'lov",
  'checkout.savedCard': 'Saqlangan karta',
  'checkout.newCard': 'Yangi karta',
  'checkout.confirm': "To'lovni tasdiqlayman:",
  'checkout.paid': "To'landi. Rahmat!",
  'account.orders': 'Buyurtmalar',
  'account.prescriptions': 'Retseptlar',
  'account.cards': 'Kartalar',
  'account.addresses': 'Manzillar',
  'account.keys': 'API kalitlar',
  'account.profile': 'Profil',
  'order.refund': "Qaytarishni so'rash",
  'order.cancel': 'Bekor qilish',
  'order.invoice': 'Hisob-faktura',
  'order.payNow': "Hozir to'lash",
  'common.loading': 'Yuklanmoqda…',
  'common.more': "Yana ko'rsatish",
  'common.save': 'Saqlash',
  'common.delete': "O'chirish",
  'common.cancel': 'Bekor qilish',
  'common.retry': 'Qayta urinish',
  'common.close': 'Yopish',
  'status.pending_payment': "To'lov kutilmoqda",
  'status.paid': "To'langan",
  'status.cancelled': 'Bekor qilingan',
  'status.partially_refunded': 'Qisman qaytarilgan',
  'status.refunded': 'Qaytarilgan',
  'status.pending_review': 'Farmatsevt tekshirmoqda',
  'status.approved': 'Tasdiqlangan',
  'status.rejected': 'Rad etilgan',
}

const dicts: Record<Lang, Dict> = { en, ru, uz }

interface I18n {
  lang: Lang
  setLang: (l: Lang) => void
  t: (k: Key) => string
}

const Ctx = createContext<I18n | null>(null)

function initialLang(): Lang {
  try {
    const saved = localStorage.getItem('dorixona.lang')
    if (saved === 'en' || saved === 'ru' || saved === 'uz') return saved
  } catch {
    /* ignore */
  }
  const nav = navigator.language.slice(0, 2)
  return nav === 'ru' || nav === 'uz' ? nav : 'en'
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(() => {
    const l = initialLang()
    session.setLang(l)
    return l
  })
  useEffect(() => {
    document.documentElement.lang = lang
  }, [lang])
  const setLang = (l: Lang) => {
    session.setLang(l)
    setLangState(l)
    try {
      localStorage.setItem('dorixona.lang', l)
    } catch {
      /* ignore */
    }
  }
  return <Ctx.Provider value={{ lang, setLang, t: (k) => dicts[lang][k] ?? en[k] }}>{children}</Ctx.Provider>
}

export function useI18n() {
  const v = useContext(Ctx)
  if (!v) throw new Error('useI18n outside I18nProvider')
  return v
}

export type TKey = Key
