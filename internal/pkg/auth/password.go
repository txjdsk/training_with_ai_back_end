package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// -------------------------- 密码通用工具--------------------------
// 生成密码的bcrypt哈希值
func GeneratePasswordHash(password string) (string, error) {
	if password == "" {
		return "", errors.New("密码不能为空")
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("密码哈希生成失败：" + err.Error())
	}
	return string(hashBytes), nil
}

// 校验明文密码是否匹配哈希值
func VerifyPassword(plainPassword, hashPassword string) (bool, error) {
	if plainPassword == "" || hashPassword == "" {
		return false, errors.New("明文密码或哈希值不能为空")
	}
	// bcrypt.CompareHashAndPassword：哈希值在前，明文在后
	err := bcrypt.CompareHashAndPassword([]byte(hashPassword), []byte(plainPassword))
	if err != nil {
		// 区分「密码不匹配」和「其他错误」（比如哈希值格式错误）
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return false, errors.New("密码错误")
		}
		return false, errors.New("密码校验失败：" + err.Error())
	}
	return true, nil
}
