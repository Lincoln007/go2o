/**
 * Copyright 2014 @ z3q.net.
 * name :
 * author : jarryliu
 * date : 2013-12-12 17:16
 * description :
 * history :
 */

package repository

import (
	"database/sql"
	"fmt"
	"github.com/jsix/gof/db"
	"go2o/core/domain/interface/member"
	"go2o/core/domain/interface/merchant"
	"go2o/core/domain/interface/merchant/shop"
	"go2o/core/domain/interface/merchant/user"
	"go2o/core/domain/interface/mss"
	"go2o/core/domain/interface/valueobject"
	merchantImpl "go2o/core/domain/merchant"
	"go2o/core/infrastructure/domain"
	"strings"
)

var _ merchant.IMerchantRep = new(merchantRep)

type merchantRep struct {
	db.Connector
	_cache   map[int]merchant.IMerchant
	_userRep user.IUserRep
	_mssRep  mss.IMssRep
	_shopRep shop.IShopRep
	_valRep  valueobject.IValueRep
}

func NewMerchantRep(c db.Connector, shopRep shop.IShopRep, userRep user.IUserRep,
	mssRep mss.IMssRep, valRep valueobject.IValueRep) merchant.IMerchantRep {
	return &merchantRep{
		Connector: c,
		_cache:    make(map[int]merchant.IMerchant),
		_userRep:  userRep,
		_mssRep:   mssRep,
		_shopRep:  shopRep,
		_valRep:   valRep,
	}
}

func (this *merchantRep) CreateMerchant(v *merchant.Merchant) (merchant.IMerchant, error) {
	return merchantImpl.NewMerchant(v, this, this._shopRep, this._userRep, this._mssRep, this._valRep)
}

func (this *merchantRep) renew(merchantId int) {
	delete(this._cache, merchantId)
}

func (this *merchantRep) GetMerchant(id int) (merchant.IMerchant, error) {
	v, ok := this._cache[id]
	var err error
	if !ok {
		e := new(merchant.Merchant)
		err = this.Connector.GetOrm().Get(id, e)
		if err == nil {
			// 缓存到列表中
			v, err = this.CreateMerchant(e)
			if v != nil {
				this._cache[id] = v
			}
		}
	}
	return v, err
}

// 获取合作商主要的域名主机
func (this *merchantRep) GetMerchantMajorHost(merchantId int) string {
	//todo:
	var host string
	this.Connector.ExecScalar(`SELECT host FROM pt_siteconf WHERE merchant_id=? LIMIT 0,1`,
		&host, merchantId)
	return host
}

// 保存
func (this *merchantRep) SaveMerchant(v *merchant.Merchant) (int, error) {
	var err error
	if v.Id <= 0 {
		orm := this.Connector.GetOrm()
		_, _, err = orm.Save(nil, v)
		err = this.Connector.ExecScalar(`SELECT MAX(id) FROM mch_merchant`, &v.Id)
		if err != nil {
			return 0, err
		}
	} else {
		_, _, err = this.Connector.GetOrm().Save(v.Id, v)
	}
	return v.Id, err
}

func (this *merchantRep) doSomething() {
	ms := []*member.Member{}
	orm := this.Connector.GetOrm()
	orm.Select(&ms, "1=1")

	for _, v := range ms {
		v.Pwd = domain.MemberSha1Pwd("123456")
		orm.Save(v.Id, v)
	}
}

// 获取商户的编号
func (this *merchantRep) GetMerchantsId() []int {

	//this.doSomething()

	dst := []int{}
	var i int

	this.Connector.Query("SELECT id FROM mch_merchant", func(rows *sql.Rows) {
		for rows.Next() {
			rows.Scan(&i)
			dst = append(dst, i)
		}
		rows.Close()
	})
	return dst
}

// 获取销售配置
func (this *merchantRep) GetMerchantSaleConf(merchantId int) *merchant.SaleConf {
	//10%分成
	//0.2,         #上级
	//0.1,         #上上级
	//0.8          #消费者自己
	var saleConf *merchant.SaleConf = new(merchant.SaleConf)
	if this.Connector.GetOrm().Get(merchantId, saleConf) == nil {
		return saleConf
	}
	return nil
}

func (this *merchantRep) SaveMerchantSaleConf(v *merchant.SaleConf) error {
	defer this.renew(v.MerchantId)
	var err error
	if v.MerchantId > 0 {
		_, _, err = this.Connector.GetOrm().Save(v.MerchantId, v)
	} else {
		_, _, err = this.Connector.GetOrm().Save(nil, v)
	}
	return err
}

// 保存API信息
func (this *merchantRep) SaveApiInfo(v *merchant.ApiInfo) error {
	orm := this.Connector.GetOrm()
	i, _, err := orm.Save(v.MerchantId, v)
	if i == 0 {
		_, _, err = orm.Save(nil, v)
	}
	return err
}

// 获取API信息
func (this *merchantRep) GetApiInfo(merchantId int) *merchant.ApiInfo {
	var d *merchant.ApiInfo = new(merchant.ApiInfo)
	if err := this.GetOrm().Get(merchantId, d); err == nil {
		return d
	}
	return nil
}

// 根据API编号获取商户编号
func (this *merchantRep) GetMerchantIdByApiId(apiId string) int {
	var merchantId int
	this.ExecScalar("SELECT merchant_id FROM mch_api_info WHERE api_id=?", &merchantId, apiId)
	return merchantId
}

// 获取键值
func (this *merchantRep) GetKeyValue(merchantId int, indent string, k string) string {
	var v string
	this.Connector.ExecScalar(
		fmt.Sprintf("SELECT value FROM pt_%s WHERE merchant_id=? AND `key`=?", indent),
		&v, merchantId, k)
	return v
}

// 设置键值
func (this *merchantRep) SaveKeyValue(merchantId int, indent string, k, v string, updateTime int64) error {
	i, err := this.Connector.ExecNonQuery(
		fmt.Sprintf("UPDATE pt_%s SET value=?,update_time=? WHERE merchant_id=? AND `key`=?", indent),
		v, updateTime, merchantId, k)
	if i == 0 {
		_, err = this.Connector.ExecNonQuery(
			fmt.Sprintf("INSERT INTO pt_%s(merchant_id,`key`,value,update_time)VALUES(?,?,?,?)", indent),
			merchantId, k, v, updateTime)
	}
	return err
}

// 获取多个键值
func (this *merchantRep) GetKeyMap(merchantId int, indent string, k []string) map[string]string {
	m := make(map[string]string)
	var k1, v1 string
	this.Connector.Query(fmt.Sprintf("SELECT `key`,value FROM pt_%s WHERE merchant_id=? AND `key` IN (?)", indent),
		func(rows *sql.Rows) {
			for rows.Next() {
				rows.Scan(&k1, &v1)
				m[k1] = v1
			}
		}, merchantId, strings.Join(k, ","))
	return m
}

// 检查是否包含值的键数量,keyStr为键模糊匹配
func (this *merchantRep) CheckKvContainValue(merchantId int, indent string, value string, keyStr string) int {
	var i int
	err := this.Connector.ExecScalar("SELECT COUNT(0) FROM pt_"+indent+
		" WHERE merchant_id=? AND value=? AND `key` LIKE '%"+
		keyStr+"%'", &i, merchantId, value)
	if err != nil {
		return 999
	}
	return i
}

// 根据关键字获取字典
func (this *merchantRep) GetKeyMapByChar(merchantId int, indent string, keyword string) map[string]string {
	m := make(map[string]string)
	var k1, v1 string
	this.Connector.Query("SELECT `key`,value FROM pt_"+indent+
		" WHERE merchant_id=? AND `key` LIKE '%"+keyword+"%'",
		func(rows *sql.Rows) {
			for rows.Next() {
				rows.Scan(&k1, &v1)
				m[k1] = v1
			}
		}, merchantId)
	return m
}
