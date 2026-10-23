import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { session } from './api'
import type { Lang } from './types'

const en = {
  'nav.shop': 'Shop',
  'nav.account': 'My pharmacy',
  'nav.staff': 'Pharmacist desk',
  'nav.admin': 'Admin',
  'nav.lab': 'Frege Lab',
  'nav.signin': 'Sign in',
  'nav.signout': 'Sign out',
  'nav.cart': 'Basket',
  'hero.kicker': 'A pharmacy built to be tested',
  'hero.title': 'Medicine, delivered across Uzbekistan.',
  'hero.sub': 'Browse a thousand made-up medicines, upload a prescription, pay with a test card and ask for refunds. Every flow is real, and every value is fake.',
  'hero.cta': 'Browse the shelves',
  'hero.lab': 'Open the lab',
  'home.categories': 'Shelves',
  'home.featured': 'Freshly stocked',
  'home.rules': 'House rules, in numbers',
  'rule.delivery': 'Free courier delivery from',
  'rule.refund': 'Refunds approved instantly up to',
  'rule.lockout': 'Wrong passwords before a 5-minute lock',
  'rule.otp': 'Minutes a sign-in code lives',
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
  'product.stockCheck': 'Check every branch',
  'product.stockCheckHint': 'A slow job: it answers 202 at once and finishes in a few seconds.',
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
  'nav.lab': 'Лаборатория Frege',
  'nav.signin': 'Войти',
  'nav.signout': 'Выйти',
  'nav.cart': 'Корзина',
  'hero.kicker': 'Аптека, созданная для тестов',
  'hero.title': 'Лекарства с доставкой по всему Узбекистану.',
  'hero.sub': 'Тысяча выдуманных лекарств, рецепты, оплата тестовой картой и возвраты. Все сценарии настоящие, все данные — нет.',
  'hero.cta': 'К полкам',
  'hero.lab': 'Открыть лабораторию',
  'home.categories': 'Полки',
  'home.featured': 'Свежие поступления',
  'home.rules': 'Правила в цифрах',
  'rule.delivery': 'Бесплатная доставка курьером от',
  'rule.refund': 'Мгновенный возврат до',
  'rule.lockout': 'Неверных паролей до блокировки на 5 минут',
  'rule.otp': 'Минут живёт код входа',
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
  'product.stockCheck': 'Проверить все филиалы',
  'product.stockCheckHint': 'Долгая задача: сразу отвечает 202 и завершается через несколько секунд.',
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
  'nav.lab': 'Frege laboratoriyasi',
  'nav.signin': 'Kirish',
  'nav.signout': 'Chiqish',
  'nav.cart': 'Savat',
  'hero.kicker': 'Sinov uchun qurilgan dorixona',
  'hero.title': "Dorilar butun O'zbekiston bo'ylab yetkaziladi.",
  'hero.sub': "Mingta o'ylab topilgan dori, retseptlar, sinov kartasi bilan to'lov va qaytarishlar. Har bir jarayon haqiqiy, har bir qiymat soxta.",
  'hero.cta': 'Javonlarga',
  'hero.lab': 'Laboratoriyani ochish',
  'home.categories': 'Javonlar',
  'home.featured': 'Yangi kelganlar',
  'home.rules': 'Qoidalar raqamlarda',
  'rule.delivery': 'Bepul kuryer yetkazish',
  'rule.refund': 'Darhol qaytarish',
  'rule.lockout': "5 daqiqalik blokdan oldingi noto'g'ri parollar",
  'rule.otp': 'Kirish kodi yashaydigan daqiqalar',
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
  'product.stockCheck': 'Barcha filiallarni tekshirish',
  'product.stockCheckHint': 'Sekin vazifa: darhol 202 qaytaradi va bir necha soniyada tugaydi.',
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
